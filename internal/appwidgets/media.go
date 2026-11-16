package appwidgets

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zekihan/sdash/config"
)

const (
	mediaCacheTTL    = 10 * time.Minute
	maxImageBytes    = 12 << 20
	maxRailItems     = 12
	hoursPerDayFloat = 24.0
	daysPerYearFloat = 365.25
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,127}$`)

// RailItem is one recently-added or memory entry with a proxied image URL.
type RailItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Image  string `json:"image"`
}

// MediaRail carries the Jellyfin and Immich rails.
type MediaRail struct {
	Jellyfin  []RailItem `json:"jellyfin"`
	Immich    []RailItem `json:"immich"`
	Available bool       `json:"available"`
}

type jellyfinUser struct {
	ID string `json:"id"`
}

type jellyfinLatestItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SeriesName     string `json:"seriesName"`
	Type           string `json:"type"`
	ProductionYear int    `json:"productionYear"`
	IndexNumber    int    `json:"indexNumber"`
	ParentIndex    int    `json:"parentIndexNumber"`
}

type immichMemory struct {
	Type   string `json:"type"`
	Assets []struct {
		ID            string    `json:"id"`
		LocalDateTime time.Time `json:"localDateTime"`
		Type          string    `json:"type"`
	} `json:"assets"`
	Data struct {
		Year int `json:"year"`
	} `json:"data"`
}

// MediaRail fetches Jellyfin recently-added and Immich on-this-day
// memories, cached briefly. Image URLs point at the dashboard's own proxy
// endpoints so upstream API keys never reach browsers.
func (provider *Provider) MediaRail(ctx context.Context) *MediaRail {
	provider.mu.Lock()
	if provider.mediaCache != nil && time.Since(provider.mediaAt) < mediaCacheTTL {
		cached := provider.mediaCache
		provider.mu.Unlock()

		return cached
	}
	provider.mu.Unlock()

	rail := new(MediaRail)
	rail.Jellyfin = []RailItem{}
	rail.Immich = []RailItem{}

	var waitGroup sync.WaitGroup

	if provider.integrations.Jellyfin.Ready() {
		waitGroup.Go(func() {
			items, err := provider.jellyfinLatest(ctx, provider.integrations.Jellyfin)
			if err != nil {
				return
			}

			provider.mu.Lock()
			rail.Jellyfin = items
			rail.Available = true
			provider.mu.Unlock()
		})
	}

	if provider.integrations.Immich.Ready() {
		waitGroup.Go(func() {
			items, err := provider.immichOnThisDay(ctx, provider.integrations.Immich)
			if err != nil {
				return
			}

			provider.mu.Lock()
			rail.Immich = items
			rail.Available = true
			provider.mu.Unlock()
		})
	}

	waitGroup.Wait()

	provider.mu.Lock()
	provider.mediaCache = rail
	provider.mediaAt = time.Now()
	provider.mu.Unlock()

	return rail
}

func (provider *Provider) jellyfinLatest(
	ctx context.Context,
	integration config.IntegrationConfig,
) ([]RailItem, error) {
	var users []jellyfinUser

	err := provider.getJSON(ctx, integration, "/Users", &users)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 || users[0].ID == "" {
		return nil, fmt.Errorf("%w: jellyfin returned no users", ErrUnavailable)
	}

	path := fmt.Sprintf(
		"/Users/%s/Items/Latest?Limit=%d&IncludeItemTypes=Movie,Episode&Fields=ProductionYear",
		users[0].ID,
		maxRailItems,
	)

	var latest []jellyfinLatestItem

	err = provider.getJSON(ctx, integration, path, &latest)
	if err != nil {
		return nil, err
	}

	items := make([]RailItem, 0, len(latest))

	for _, entry := range latest {
		item := new(RailItem)
		item.ID = entry.ID
		item.Title = entry.Name
		item.Image = "/api/jellyfin/image/" + entry.ID

		if entry.SeriesName != "" {
			item.Title = entry.SeriesName
			item.Detail = fmt.Sprintf(
				"S%02dE%02d · %s",
				entry.ParentIndex,
				entry.IndexNumber,
				entry.Name,
			)
		} else if entry.ProductionYear > 0 {
			item.Detail = strconv.Itoa(entry.ProductionYear)
		}

		items = append(items, *item)
	}

	return items, nil
}

func (provider *Provider) immichOnThisDay(
	ctx context.Context,
	integration config.IntegrationConfig,
) ([]RailItem, error) {
	var memories []immichMemory

	err := provider.getJSON(ctx, integration, "/api/memories?type=on_this_day", &memories)
	if err != nil {
		return nil, err
	}

	items := make([]RailItem, 0, maxRailItems)
	now := time.Now()

	for _, memory := range memories {
		for _, asset := range memory.Assets {
			if asset.Type != "" && asset.Type != "IMAGE" {
				continue
			}

			item := new(RailItem)
			item.ID = asset.ID
			item.Image = "/api/immich/thumbnail/" + asset.ID

			years := memory.Data.Year
			if years <= 0 && !asset.LocalDateTime.IsZero() {
				years = int(
					now.Sub(asset.LocalDateTime).Hours() / hoursPerDayFloat / daysPerYearFloat,
				)
			}

			switch {
			case years == 1:
				item.Title = "1 year ago"
			case years > 1:
				item.Title = fmt.Sprintf("%d years ago", years)
			default:
				item.Title = "On this day"
			}

			if !asset.LocalDateTime.IsZero() {
				item.Detail = asset.LocalDateTime.Format("Jan 2, 2006")
			}

			items = append(items, *item)
			if len(items) >= maxRailItems {
				return items, nil
			}
		}
	}

	return items, nil
}

// JellyfinImage proxies one item's primary image (poster/backdrop size).
func (provider *Provider) JellyfinImage(
	ctx context.Context,
	itemID string,
) ([]byte, string, error) {
	if !safeIDPattern.MatchString(itemID) {
		return nil, "", fmt.Errorf("%w: invalid item id", ErrUnavailable)
	}

	integration := provider.integrations.Jellyfin

	return provider.fetchImage(ctx, integration, "/Items/"+itemID+"/Images/Primary?maxHeight=480")
}

// ImmichThumbnail proxies one asset thumbnail or preview image.
func (provider *Provider) ImmichThumbnail(
	ctx context.Context,
	assetID, size string,
) ([]byte, string, error) {
	if !safeIDPattern.MatchString(assetID) {
		return nil, "", fmt.Errorf("%w: invalid asset id", ErrUnavailable)
	}

	if size != "preview" {
		size = "thumbnail"
	}

	integration := provider.integrations.Immich

	return provider.fetchImage(ctx, integration, "/api/assets/"+assetID+"/thumbnail?size="+size)
}

func (provider *Provider) fetchImage(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath string,
) ([]byte, string, error) {
	if !integration.Ready() {
		return nil, "", fmt.Errorf("%w: integration not configured", ErrUnavailable)
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	fullURL := strings.TrimSuffix(integration.URL, "/") + apiPath

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fullURL, http.NoBody)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("X-Api-Key", integration.APIKey)
	req.Header.Set("X-Emby-Token", integration.APIKey)

	resp, err := provider.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf(
			"%w: image fetch returned %s (%s)",
			ErrUnavailable,
			resp.Status,
			contentType,
		)
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	return content, contentType, nil
}
