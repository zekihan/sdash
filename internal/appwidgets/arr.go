package appwidgets

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/zekihan/sdash/config"
)

const maxLookupResults = 8

// LookupResult is one Sonarr/Radarr search hit.
type LookupResult struct {
	Title      string `json:"title"`
	Overview   string `json:"overview"`
	Poster     string `json:"poster"`
	ExternalID int    `json:"externalId"`
	Year       int    `json:"year"`
	Added      bool   `json:"added"`
}

type arrLookupEntry struct {
	Title        string `json:"title"`
	Overview     string `json:"overview"`
	RemotePoster string `json:"remotePoster"`
	TVDBID       int    `json:"tvdbId"`
	TMDBID       int    `json:"tmdbId"`
	Year         int    `json:"year"`
	ID           int    `json:"id"`
}

type arrProfile struct {
	ID int `json:"id"`
}

type arrRootFolder struct {
	Path string `json:"path"`
}

// Lookup searches Sonarr or Radarr for addable titles.
func (provider *Provider) Lookup(
	ctx context.Context,
	arrType, term string,
) ([]LookupResult, error) {
	integration, ok := provider.integrationFor(arrType)
	if !ok || (arrType != typeSonarr && arrType != typeRadarr) {
		return nil, fmt.Errorf("%w: %s not configured", ErrUnavailable, arrType)
	}

	var path string
	if arrType == "sonarr" {
		path = "/api/v3/series/lookup?term=" + url.QueryEscape(term)
	} else {
		path = "/api/v3/movie/lookup?term=" + url.QueryEscape(term)
	}

	var entries []arrLookupEntry

	err := provider.getJSON(ctx, integration, path, &entries)
	if err != nil {
		return nil, err
	}

	results := make([]LookupResult, 0, len(entries))

	for _, entry := range entries {
		result := new(LookupResult)
		result.Title = entry.Title
		result.Overview = entry.Overview
		result.Poster = entry.RemotePoster
		result.Year = entry.Year
		result.Added = entry.ID > 0

		if arrType == "sonarr" {
			result.ExternalID = entry.TVDBID
		} else {
			result.ExternalID = entry.TMDBID
		}

		if result.ExternalID == 0 {
			continue
		}

		results = append(results, *result)
		if len(results) >= maxLookupResults {
			break
		}
	}

	return results, nil
}

// AddTitle adds a looked-up movie/show with the first quality profile and
// root folder, monitored, searching immediately.
func (provider *Provider) AddTitle(ctx context.Context, arrType string, externalID int) error {
	integration, ok := provider.integrationFor(arrType)
	if !ok || (arrType != typeSonarr && arrType != typeRadarr) {
		return fmt.Errorf("%w: %s not configured", ErrUnavailable, arrType)
	}

	qualityProfileID, err := provider.firstArrID(ctx, integration, "/api/v3/qualityprofile")
	if err != nil {
		return err
	}

	rootFolder, err := provider.firstRootFolder(ctx, integration)
	if err != nil {
		return err
	}

	if arrType == typeRadarr {
		return provider.addMovie(ctx, integration, externalID, qualityProfileID, rootFolder)
	}

	return provider.addSeries(ctx, integration, externalID, qualityProfileID, rootFolder)
}

func (provider *Provider) addMovie(
	ctx context.Context,
	integration config.IntegrationConfig,
	tmdbID, qualityProfileID int,
	rootFolder string,
) error {
	lookup := make(map[string]any)

	path := fmt.Sprintf("/api/v3/movie/lookup/tmdb?tmdbId=%d", tmdbID)

	err := provider.getJSON(ctx, integration, path, &lookup)
	if err != nil {
		return err
	}

	lookup["qualityProfileId"] = qualityProfileID
	lookup["rootFolderPath"] = rootFolder
	lookup["monitored"] = true
	lookup["minimumAvailability"] = "announced"
	lookup["addOptions"] = map[string]any{"searchForMovie": true}

	return provider.postJSONBody(ctx, integration, "/api/v3/movie", lookup)
}

func (provider *Provider) addSeries(
	ctx context.Context,
	integration config.IntegrationConfig,
	tvdbID, qualityProfileID int,
	rootFolder string,
) error {
	var results []map[string]any

	path := fmt.Sprintf("/api/v3/series/lookup?term=tvdb:%d", tvdbID)

	err := provider.getJSON(ctx, integration, path, &results)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("%w: series %d not found", ErrUnavailable, tvdbID)
	}

	series := results[0]
	series["qualityProfileId"] = qualityProfileID
	series["rootFolderPath"] = rootFolder
	series["monitored"] = true
	series["seasonFolder"] = true
	series["addOptions"] = map[string]any{"searchForMissingEpisodes": true}

	languageProfileID, langErr := provider.firstArrID(ctx, integration, "/api/v3/languageprofile")
	if langErr == nil && languageProfileID > 0 {
		series["languageProfileId"] = languageProfileID
	}

	return provider.postJSONBody(ctx, integration, "/api/v3/series", series)
}

func (provider *Provider) firstArrID(
	ctx context.Context,
	integration config.IntegrationConfig,
	path string,
) (int, error) {
	var profiles []arrProfile

	err := provider.getJSON(ctx, integration, path, &profiles)
	if err != nil {
		return 0, err
	}

	if len(profiles) == 0 {
		return 0, fmt.Errorf("%w: no entries at %s", ErrUnavailable, path)
	}

	return profiles[0].ID, nil
}

func (provider *Provider) firstRootFolder(
	ctx context.Context,
	integration config.IntegrationConfig,
) (string, error) {
	var folders []arrRootFolder

	err := provider.getJSON(ctx, integration, "/api/v3/rootfolder", &folders)
	if err != nil {
		return "", err
	}

	if len(folders) == 0 || folders[0].Path == "" {
		return "", fmt.Errorf("%w: no root folder configured", ErrUnavailable)
	}

	return folders[0].Path, nil
}

// AddMagnet adds a magnet link (or URL) to qBittorrent.
func (provider *Provider) AddMagnet(ctx context.Context, magnet string) error {
	integration := provider.integrations.QBittorrent
	if !integration.Ready() {
		return fmt.Errorf("%w: qbittorrent not configured", ErrUnavailable)
	}

	if !strings.HasPrefix(magnet, "magnet:?") && !strings.HasPrefix(magnet, "http") {
		return fmt.Errorf("%w: not a magnet link or URL", ErrBadResponse)
	}

	cookie, err := provider.qbitLogin(ctx, integration)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("urls", magnet)

	return provider.postFormCookie(ctx, integration, "/api/v2/torrents/add", cookie, form)
}
