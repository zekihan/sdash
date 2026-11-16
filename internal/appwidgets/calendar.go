package appwidgets

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/zekihan/sdash/config"
)

const (
	dayDuration   = 24 * time.Hour
	calendarDays  = 7
	calendarCache = 5 * time.Minute
)

// CalendarItem is one upcoming episode or movie release.
type CalendarItem struct {
	AirDate time.Time `json:"airDate"`
	Title   string    `json:"title"`
	Detail  string    `json:"detail"`
	Source  string    `json:"source"`
	HasFile bool      `json:"hasFile"`
}

// CalendarResponse is the coming-this-week rail payload.
type CalendarResponse struct {
	Items     []CalendarItem `json:"items"`
	Available bool           `json:"available"`
}

type sonarrCalendarEpisode struct {
	AirDateUTC time.Time `json:"airDateUtc"`
	Series     struct {
		Title string `json:"title"`
	} `json:"series"`
	Title         string `json:"title"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	HasFile       bool   `json:"hasFile"`
}

type radarrCalendarMovie struct {
	DigitalRelease  *time.Time `json:"digitalRelease"`
	PhysicalRelease *time.Time `json:"physicalRelease"`
	InCinemas       *time.Time `json:"inCinemas"`
	Title           string     `json:"title"`
	Year            int        `json:"year"`
	HasFile         bool       `json:"hasFile"`
}

// Calendar returns the next week's Sonarr episodes and Radarr releases,
// cached briefly. Unconfigured sources are skipped.
func (provider *Provider) Calendar(ctx context.Context) *CalendarResponse {
	provider.mu.Lock()
	if provider.calendarCache != nil && time.Since(provider.calendarAt) < calendarCache {
		cached := provider.calendarCache
		provider.mu.Unlock()

		return cached
	}
	provider.mu.Unlock()

	response := new(CalendarResponse)
	response.Items = []CalendarItem{}

	start := time.Now().Truncate(dayDuration)
	end := start.Add(calendarDays * dayDuration)

	var (
		mu        sync.Mutex
		waitGroup sync.WaitGroup
	)

	collect := func(fetch func() ([]CalendarItem, error)) {
		waitGroup.Go(func() {
			items, err := fetch()
			if err != nil {
				return
			}

			mu.Lock()

			response.Items = append(response.Items, items...)
			response.Available = true
			mu.Unlock()
		})
	}

	if provider.integrations.Sonarr.Ready() {
		collect(func() ([]CalendarItem, error) {
			return provider.sonarrCalendar(ctx, provider.integrations.Sonarr, start, end)
		})
	}

	if provider.integrations.Radarr.Ready() {
		collect(func() ([]CalendarItem, error) {
			return provider.radarrCalendar(ctx, provider.integrations.Radarr, start, end)
		})
	}

	waitGroup.Wait()

	sort.Slice(response.Items, func(first, second int) bool {
		return response.Items[first].AirDate.Before(response.Items[second].AirDate)
	})

	provider.mu.Lock()
	provider.calendarCache = response
	provider.calendarAt = time.Now()
	provider.mu.Unlock()

	return response
}

func (provider *Provider) sonarrCalendar(
	ctx context.Context,
	integration config.IntegrationConfig,
	start, end time.Time,
) ([]CalendarItem, error) {
	path := fmt.Sprintf("/api/v3/calendar?start=%s&end=%s&includeSeries=true",
		start.Format(time.DateOnly), end.Format(time.DateOnly))

	var episodes []sonarrCalendarEpisode

	err := provider.getJSON(ctx, integration, path, &episodes)
	if err != nil {
		return nil, err
	}

	items := make([]CalendarItem, 0, len(episodes))

	for _, episode := range episodes {
		item := new(CalendarItem)
		item.Title = episode.Series.Title
		item.Detail = fmt.Sprintf(
			"S%02dE%02d · %s",
			episode.SeasonNumber,
			episode.EpisodeNumber,
			episode.Title,
		)
		item.Source = typeSonarr
		item.AirDate = episode.AirDateUTC
		item.HasFile = episode.HasFile
		items = append(items, *item)
	}

	return items, nil
}

func (provider *Provider) radarrCalendar(
	ctx context.Context,
	integration config.IntegrationConfig,
	start, end time.Time,
) ([]CalendarItem, error) {
	path := fmt.Sprintf("/api/v3/calendar?start=%s&end=%s",
		start.Format(time.DateOnly), end.Format(time.DateOnly))

	var movies []radarrCalendarMovie

	err := provider.getJSON(ctx, integration, path, &movies)
	if err != nil {
		return nil, err
	}

	items := make([]CalendarItem, 0, len(movies))

	for _, movie := range movies {
		release, kind := movieRelease(movie, start, end)
		if release.IsZero() {
			continue
		}

		item := new(CalendarItem)
		item.Title = movie.Title

		item.Detail = kind
		if movie.Year > 0 {
			item.Detail = fmt.Sprintf("%s · %d", kind, movie.Year)
		}

		item.Source = typeRadarr
		item.AirDate = release
		item.HasFile = movie.HasFile
		items = append(items, *item)
	}

	return items, nil
}

func movieRelease(movie radarrCalendarMovie, start, end time.Time) (time.Time, string) {
	// Prefer the release that actually makes the movie watchable at home.
	for _, candidate := range []struct {
		at   *time.Time
		kind string
	}{
		{movie.DigitalRelease, "digital release"},
		{movie.PhysicalRelease, "physical release"},
		{movie.InCinemas, "in cinemas"},
	} {
		if candidate.at != nil && !candidate.at.Before(start) && candidate.at.Before(end) {
			return *candidate.at, candidate.kind
		}
	}

	return time.Time{}, ""
}
