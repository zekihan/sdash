package kubeinfo

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLonghornNamespaceFallback(t *testing.T) {
	const (
		emptyList  = `{"items":[]}`
		volumeList = `{"items":[{"metadata":{"name":"pvc-demo"},` +
			`"status":{"state":"attached","robustness":"degraded"}}]}`
		volumesPath = "/apis/longhorn.io/v1beta2/namespaces/"
	)

	tests := []struct {
		name           string
		longhorn       string
		longhornSystem string
		wantVolume     bool
	}{
		{"empty longhorn", emptyList, volumeList, true},
		{"forbidden longhorn", "", volumeList, true},
		{"populated longhorn", volumeList, "", true},
		{"empty longhorn with forbidden fallback", emptyList, "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body string

					switch r.URL.Path {
					case volumesPath + "longhorn/volumes":
						body = test.longhorn
					case volumesPath + "longhorn-system/volumes":
						body = test.longhornSystem
					default:
						http.NotFound(w, r)

						return
					}

					if body == "" {
						w.WriteHeader(http.StatusForbidden)

						return
					}

					_, _ = io.WriteString(w, body)
				}),
			)
			t.Cleanup(server.Close)

			client := new(Client)
			client.httpClient = server.Client()
			client.base = server.URL

			summary := new(Summary)

			err := client.fetchLonghorn(t.Context(), summary)
			if err != nil {
				t.Fatalf("fetchLonghorn: %v", err)
			}

			got := summary.Longhorn
			if !got.Available {
				t.Fatal("Longhorn summary is unavailable")
			}

			if !test.wantVolume {
				if got.Total != 0 || len(got.Attention) != 0 {
					t.Errorf("Longhorn summary = %+v, want empty", got)
				}

				return
			}

			if got.Total != 1 || got.Degraded != 1 || len(got.Attention) != 1 ||
				got.Attention[0].Name != "pvc-demo" {
				t.Errorf("Longhorn summary = %+v, want one degraded pvc-demo volume", got)
			}
		})
	}
}
