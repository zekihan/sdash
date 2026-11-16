package systeminfo

import (
	"runtime"
	"testing"
)

func TestRead(t *testing.T) {
	info := Read()

	if info.OS != runtime.GOOS || info.CPUs < 1 {
		t.Fatalf("unexpected identity: %+v", info)
	}

	if runtime.GOOS == "linux" {
		if !info.Available {
			t.Fatal("linux stats should be available")
		}

		if info.MemTotal == 0 || info.MemUsed > info.MemTotal {
			t.Errorf("bad memory reading: total=%d used=%d", info.MemTotal, info.MemUsed)
		}

		if info.DiskTotal == 0 || info.DiskUsed > info.DiskTotal {
			t.Errorf("bad disk reading: total=%d used=%d", info.DiskTotal, info.DiskUsed)
		}

		if info.UptimeSec <= 0 {
			t.Errorf("bad uptime: %d", info.UptimeSec)
		}
	}
}
