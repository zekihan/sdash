package systeminfo

import "testing"

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
