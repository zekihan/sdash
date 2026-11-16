// Package weather fetches current conditions and a short forecast from
// Open-Meteo (no API key required) server-side and caches them, so browsers
// never call third parties directly and the dashboard works air-gapped
// apart from this one outbound call.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/zekihan/sdash/config"
)

const (
	cacheTTL         = 15 * time.Minute
	fetchTimeout     = 6 * time.Second
	maxResponseBytes = 1 << 20
)

// WMO weather-code boundaries used by Describe.
const (
	codeFogStart         = 45
	codeFogEnd           = 48
	codeDrizzleStart     = 51
	codeDrizzleEnd       = 57
	codeRainStart        = 61
	codeRainEnd          = 67
	codeSnowStart        = 71
	codeSnowEnd          = 77
	codeShowersStart     = 80
	codeShowersEnd       = 82
	codeSnowShowersStart = 85
	codeSnowShowersEnd   = 86
	codeThunderstorm     = 95
)

var (
	ErrUnavailable = errors.New("weather service unavailable")
	ErrBadResponse = errors.New("invalid weather response")
)

// Report is the dashboard-facing weather summary.
type Report struct {
	Label       string  `json:"label"`
	Units       string  `json:"units"`
	Description string  `json:"description"`
	FetchedAt   string  `json:"fetchedAt"`
	Daily       []Day   `json:"daily"`
	Temp        float64 `json:"temperature"`
	FeelsLike   float64 `json:"feelsLike"`
	Humidity    float64 `json:"humidity"`
	WindSpeed   float64 `json:"windSpeed"`
	Code        int     `json:"code"`
	IsDay       bool    `json:"isDay"`
}

// Day is one forecast day.
type Day struct {
	Date string  `json:"date"`
	Max  float64 `json:"max"`
	Min  float64 `json:"min"`
	Code int     `json:"code"`
}

// apiResponse mirrors the Open-Meteo forecast payload. Field names are
// dictated by that API.
//
//nolint:tagliatelle // Open-Meteo uses snake_case keys
type apiResponse struct {
	Daily struct {
		Time []string  `json:"time"`
		Code []int     `json:"weather_code"`
		Max  []float64 `json:"temperature_2m_max"`
		Min  []float64 `json:"temperature_2m_min"`
	} `json:"daily"`
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		FeelsLike   float64 `json:"apparent_temperature"`
		Humidity    float64 `json:"relative_humidity_2m"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		Code        int     `json:"weather_code"`
		IsDay       int     `json:"is_day"`
	} `json:"current"`
}

// Client caches the last successful report.
type Client struct {
	at         time.Time
	httpClient *http.Client
	cached     *Report
	cfg        config.WeatherConfig
	mu         sync.Mutex
}

// New creates a weather client for the configured location.
func New(cfg config.WeatherConfig) *Client {
	httpClient := new(http.Client)
	httpClient.Timeout = fetchTimeout

	client := new(Client)
	client.cfg = cfg
	client.httpClient = httpClient

	return client
}

// Current returns the cached report when fresh, refetching otherwise. A
// failed refetch falls back to the last good report when one exists.
func (client *Client) Current(ctx context.Context) (*Report, error) {
	client.mu.Lock()
	if client.cached != nil && time.Since(client.at) < cacheTTL {
		report := client.cached
		client.mu.Unlock()

		return report, nil
	}
	client.mu.Unlock()

	report, err := client.fetch(ctx)
	if err != nil {
		client.mu.Lock()
		defer client.mu.Unlock()

		if client.cached != nil {
			return client.cached, nil
		}

		return nil, err
	}

	client.mu.Lock()
	client.cached, client.at = report, time.Now()
	client.mu.Unlock()

	return report, nil
}

func (client *Client) fetch(ctx context.Context) (*Report, error) {
	tempUnit, windUnit := "celsius", "kmh"
	if client.cfg.Units == "imperial" {
		tempUnit, windUnit = "fahrenheit", "mph"
	}

	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f"+
			"&current=temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,weather_code,is_day"+
			"&daily=weather_code,temperature_2m_max,temperature_2m_min&forecast_days=5"+
			"&temperature_unit=%s&wind_speed_unit=%s&timezone=auto",
		client.cfg.Latitude, client.cfg.Longitude, tempUnit, windUnit,
	)

	reqCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("User-Agent", "sdash/1.0")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	parsed := new(apiResponse)

	err = json.Unmarshal(body, parsed)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	return client.toReport(parsed), nil
}

func (client *Client) toReport(parsed *apiResponse) *Report {
	report := new(Report)
	report.Label = client.cfg.Label
	report.Units = client.cfg.Units
	report.Temp = parsed.Current.Temperature
	report.FeelsLike = parsed.Current.FeelsLike
	report.Humidity = parsed.Current.Humidity
	report.WindSpeed = parsed.Current.WindSpeed
	report.Code = parsed.Current.Code
	report.Description = Describe(parsed.Current.Code)
	report.IsDay = parsed.Current.IsDay == 1
	report.FetchedAt = time.Now().Format(time.RFC3339)
	report.Daily = make([]Day, 0, len(parsed.Daily.Time))

	for index, date := range parsed.Daily.Time {
		day := new(Day)
		day.Date = date

		if index < len(parsed.Daily.Code) {
			day.Code = parsed.Daily.Code[index]
		}

		if index < len(parsed.Daily.Max) {
			day.Max = parsed.Daily.Max[index]
		}

		if index < len(parsed.Daily.Min) {
			day.Min = parsed.Daily.Min[index]
		}

		report.Daily = append(report.Daily, *day)
	}

	return report
}

// Describe maps a WMO weather code to a short label.
//
//nolint:gocyclo // flat lookup over the WMO code table
func Describe(code int) string {
	switch {
	case code == 0:
		return "Clear"
	case code == 1:
		return "Mostly clear"
	case code == 2: //nolint:mnd // WMO code 2: partly cloudy
		return "Partly cloudy"
	case code == 3: //nolint:mnd // WMO code 3: overcast
		return "Overcast"
	case code >= codeFogStart && code <= codeFogEnd:
		return "Fog"
	case code >= codeDrizzleStart && code <= codeDrizzleEnd:
		return "Drizzle"
	case code >= codeRainStart && code <= codeRainEnd:
		return "Rain"
	case code >= codeSnowStart && code <= codeSnowEnd:
		return "Snow"
	case code >= codeShowersStart && code <= codeShowersEnd:
		return "Showers"
	case code >= codeSnowShowersStart && code <= codeSnowShowersEnd:
		return "Snow showers"
	case code >= codeThunderstorm:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}
