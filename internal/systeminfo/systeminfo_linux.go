package systeminfo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	percentScale  = 100.0
	bytesPerKiB   = 1024
	cpuFieldCount = 5
	idleField     = 3
	iowaitField   = 4
	guestField    = 8
	memFieldCount = 2
	loadFieldCnt  = 3
	floatBitSize  = 64
	uintBitSize   = 64
	decimalBase   = 10
)

var (
	errNoCPULine  = errors.New("no aggregate cpu line in /proc/stat")
	errEmptyProc  = errors.New("empty /proc file")
	errBadStatfs  = errors.New("statfs returned no blocks")
	errBadBlockSz = errors.New("statfs returned a non-positive block size")
)

type cpuTimes struct {
	idle  uint64
	total uint64
}

var (
	cpuMu   sync.Mutex
	lastCPU cpuTimes
	hasCPU  bool
)

// readPlatform fills info from /proc and statfs.
func readPlatform(info *Info) {
	info.Available = true
	info.CPUPercent = cpuPercent()
	readMem(info)
	readLoad(info)
	readDisk(info)

	uptime, err := readUptime()
	if err == nil {
		info.UptimeSec = uptime
	}
}

func cpuPercent() float64 {
	current, err := readCPUTimes()
	if err != nil {
		return 0
	}

	cpuMu.Lock()
	defer cpuMu.Unlock()

	previous, had := lastCPU, hasCPU
	lastCPU, hasCPU = current, true

	if !had {
		return 0
	}

	return cpuUtilization(previous, current)
}

func cpuUtilization(previous, current cpuTimes) float64 {
	// Linux iowait can decrease, which can make the combined idle counter go backwards.
	if current.total <= previous.total || current.idle < previous.idle {
		return 0
	}

	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle

	if idleDelta > totalDelta {
		return 0
	}

	return float64(totalDelta-idleDelta) / float64(totalDelta) * percentScale
}

func readCPUTimes() (cpuTimes, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return cpuTimes{}, fmt.Errorf("open /proc/stat: %w", err)
	}
	defer file.Close()

	return parseCPUTimes(file)
}

func parseCPUTimes(reader io.Reader) (cpuTimes, error) {
	var times cpuTimes

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < cpuFieldCount || fields[0] != "cpu" {
			continue
		}

		// Linux already includes guest in user and guest_nice in nice.
		counters := fields[1:]
		for index, field := range counters[:min(len(counters), guestField)] {
			value, parseErr := strconv.ParseUint(field, decimalBase, uintBitSize)
			if parseErr != nil {
				continue
			}

			times.total += value
			if index == idleField || index == iowaitField {
				times.idle += value
			}
		}

		return times, nil
	}

	return times, errNoCPULine
}

func readMem(info *Info) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer file.Close()

	var total, available uint64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < memFieldCount {
			continue
		}

		value, parseErr := strconv.ParseUint(fields[1], decimalBase, uintBitSize)
		if parseErr != nil {
			continue
		}

		switch fields[0] {
		case "MemTotal:":
			total = value * bytesPerKiB
		case "MemAvailable:":
			available = value * bytesPerKiB
		}
	}

	if total > 0 && available <= total {
		info.MemTotal = total
		info.MemUsed = total - available
		info.MemPercent = float64(info.MemUsed) / float64(total) * percentScale
	}
}

func readLoad(info *Info) {
	content, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}

	fields := strings.Fields(string(content))
	if len(fields) < loadFieldCnt {
		return
	}

	info.Load1 = parseFloatOrZero(fields[0])
	info.Load5 = parseFloatOrZero(fields[1])
	info.Load15 = parseFloatOrZero(fields[2])
}

func parseFloatOrZero(raw string) float64 {
	value, err := strconv.ParseFloat(raw, floatBitSize)
	if err != nil {
		return 0
	}

	return value
}

func readUptime() (int64, error) {
	content, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("read /proc/uptime: %w", err)
	}

	fields := strings.Fields(string(content))
	if len(fields) < 1 {
		return 0, errEmptyProc
	}

	seconds, err := strconv.ParseFloat(fields[0], floatBitSize)
	if err != nil {
		return 0, fmt.Errorf("parse /proc/uptime: %w", err)
	}

	return int64(seconds), nil
}

func readDisk(info *Info) {
	stat := new(unix.Statfs_t)

	err := unix.Statfs(info.DiskPath, stat)
	if err != nil || stat.Blocks == 0 {
		return
	}

	blockSize, err := blockSizeBytes(stat)
	if err != nil {
		return
	}

	info.DiskTotal = stat.Blocks * blockSize
	free := stat.Bavail * blockSize
	info.DiskUsed = info.DiskTotal - free
	info.DiskPct = float64(info.DiskUsed) / float64(info.DiskTotal) * percentScale
}

func blockSizeBytes(stat *unix.Statfs_t) (uint64, error) {
	if stat.Blocks == 0 {
		return 0, errBadStatfs
	}

	size := stat.Bsize
	if size <= 0 {
		return 0, errBadBlockSz
	}

	return uint64(size), nil
}
