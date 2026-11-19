package weather

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zekihan/sdash/config"
)

const completeForecast = `{"current":{
	"temperature_2m":0,
	"apparent_temperature":0,
	"relative_humidity_2m":0,
	"wind_speed_10m":0,
	"weather_code":0,
	"is_day":0
}}`

type forecastTransport struct {
	body string
}

func (fixture *forecastTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Body = io.NopCloser(strings.NewReader(fixture.body))

	return response, nil
}

func TestCurrentRejectsIncompleteConditions(t *testing.T) {
	cases := map[string]string{
		"missing current":     `{"daily":{"time":["2026-11-19"]}}`,
		"null current":        `{"current":null}`,
		"missing temperature": strings.Replace(completeForecast, `"temperature_2m":0,`, "", 1),
		"null temperature": strings.Replace(
			completeForecast,
			`"temperature_2m":0`,
			`"temperature_2m":null`,
			1,
		),
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := &forecastTransport{body: body}
			cfg := new(config.WeatherConfig)
			client := New(*cfg)
			client.httpClient.Transport = fixture

			report, err := client.Current(t.Context())
			if report != nil || !errors.Is(err, ErrBadResponse) {
				t.Fatalf("Current = %+v, %v, want no report and ErrBadResponse", report, err)
			}
		})
	}
}

func TestCurrentKeepsLastGoodConditions(t *testing.T) {
	fixture := &forecastTransport{body: completeForecast}
	cfg := new(config.WeatherConfig)
	client := New(*cfg)
	client.httpClient.Transport = fixture

	good, err := client.Current(t.Context())
	if err != nil {
		t.Fatalf("Current rejected valid zero readings: %v", err)
	}

	client.at = time.Time{}
	fixture.body = `{"current":null}`

	report, err := client.Current(t.Context())
	if err != nil || report != good {
		t.Fatalf("Current = %+v, %v, want cached report %+v", report, err, good)
	}
}

func TestDescribe(t *testing.T) {
	cases := map[int]string{
		0:  "Clear",
		3:  "Overcast",
		61: "Rain",
		71: "Snow",
		95: "Thunderstorm",
		42: "Unknown",
	}

	for code, want := range cases {
		if got := Describe(code); got != want {
			t.Errorf("Describe(%d) = %q, want %q", code, got, want)
		}
	}
}
