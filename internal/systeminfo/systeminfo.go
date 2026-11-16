// Package systeminfo reads host CPU, memory, disk and load statistics. On
// hosts where they cannot be collected every field degrades to unavailable
// instead of failing, so the widget simply hides.
package systeminfo

import (
	"os"
	"runtime"
)

// Info is one system-stats reading.
type Info struct {
	Hostname   string  `json:"hostname"`
	OS         string  `json:"os"`
	Arch       string  `json:"arch"`
	DiskPath   string  `json:"diskPath"`
	CPUPercent float64 `json:"cpuPercent"`
	MemPercent float64 `json:"memPercent"`
	DiskPct    float64 `json:"diskPercent"`
	Load1      float64 `json:"load1"`
	Load5      float64 `json:"load5"`
	Load15     float64 `json:"load15"`
	MemTotal   uint64  `json:"memTotal"`
	MemUsed    uint64  `json:"memUsed"`
	DiskTotal  uint64  `json:"diskTotal"`
	DiskUsed   uint64  `json:"diskUsed"`
	UptimeSec  int64   `json:"uptimeSec"`
	CPUs       int     `json:"cpus"`
	Available  bool    `json:"available"`
}

// Read collects the current stats. CPU percent is measured against the
// previous call, so the first reading reports 0.
func Read() Info {
	info := new(Info)
	info.OS = runtime.GOOS
	info.Arch = runtime.GOARCH
	info.CPUs = runtime.NumCPU()
	info.DiskPath = "/"

	hostname, err := os.Hostname()
	if err == nil {
		info.Hostname = hostname
	}

	readPlatform(info)

	return *info
}
