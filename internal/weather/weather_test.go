package weather

import "testing"

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
