package systeminfo

import (
	"strings"
	"testing"
)

func TestParseCPUTimes(t *testing.T) {
	var baseline cpuTimes

	cases := []struct {
		name    string
		stat    string
		want    cpuTimes
		wantPct float64
	}{
		{
			name:    "guest time included in user",
			stat:    "cpu 50 0 0 50 0 0 0 0 50 0\ncpu0 50 0 0 50 0 0 0 0 50 0\n",
			want:    cpuTimes{idle: 50, total: 100},
			wantPct: 50,
		},
		{
			name:    "guest nice time included in nice",
			stat:    "cpu 10 40 0 50 0 0 0 0 0 30\n",
			want:    cpuTimes{idle: 50, total: 100},
			wantPct: 50,
		},
		{
			name:    "zero guest counters with iowait",
			stat:    "cpu 20 1 7 45 15 3 4 5 0 0\n",
			want:    cpuTimes{idle: 60, total: 100},
			wantPct: 40,
		},
		{
			name:    "legacy row without iowait or guest counters",
			stat:    "cpu 20 5 25 50\n",
			want:    cpuTimes{idle: 50, total: 100},
			wantPct: 50,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			times, err := parseCPUTimes(strings.NewReader(testCase.stat))
			if err != nil {
				t.Fatal(err)
			}

			if times != testCase.want {
				t.Errorf("CPU times = %+v, want %+v", times, testCase.want)
			}

			if got := cpuUtilization(baseline, times); got != testCase.wantPct {
				t.Errorf("CPU utilization = %g%%, want %g%%", got, testCase.wantPct)
			}
		})
	}
}

func TestCPUUtilization(t *testing.T) {
	previous := cpuTimes{idle: 100, total: 200}
	cases := []struct {
		name    string
		current cpuTimes
		want    float64
	}{
		{name: "valid interval", current: cpuTimes{idle: 110, total: 220}, want: 50},
		{name: "idle counter decreased", current: cpuTimes{idle: 90, total: 210}, want: 0},
		{
			name:    "idle increase exceeds total increase",
			current: cpuTimes{idle: 120, total: 210},
			want:    0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := cpuUtilization(previous, testCase.current); got != testCase.want {
				t.Errorf("CPU utilization = %g%%, want %g%%", got, testCase.want)
			}
		})
	}
}
