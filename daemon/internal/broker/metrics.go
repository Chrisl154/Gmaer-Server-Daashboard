package broker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const metricsBufferSize = 60 // 15 min at 15 s intervals

// ServerMetricSample holds a single resource utilisation snapshot for a server.
type ServerMetricSample struct {
	Timestamp   int64   `json:"ts"`
	CPUPercent  float64 `json:"cpu_pct"`
	RAMPercent  float64 `json:"ram_pct"`
	DiskPercent float64 `json:"disk_pct"`
	PlayerCount int     `json:"player_count"`
	NetInKbps   float64 `json:"net_in_kbps,omitempty"`
	NetOutKbps  float64 `json:"net_out_kbps,omitempty"`
}

// diskUsagePct returns the percentage of the filesystem partition that is used
// by the given path. Returns 0 on any error or on non-Linux platforms.
func diskUsagePct(path string) float64 {
	if path == "" || runtime.GOOS != "linux" {
		return 0
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	if stat.Blocks == 0 {
		return 0
	}
	used := stat.Blocks - stat.Bfree
	return float64(used) / float64(stat.Blocks) * 100.0
}

// metricsRing is a thread-safe fixed-capacity ring buffer of ServerMetricSamples.
type metricsRing struct {
	mu      sync.RWMutex
	samples [metricsBufferSize]ServerMetricSample
	head    int
	count   int
}

func (r *metricsRing) push(s ServerMetricSample) {
	r.mu.Lock()
	r.samples[r.head%metricsBufferSize] = s
	r.head++
	if r.count < metricsBufferSize {
		r.count++
	}
	r.mu.Unlock()
}

// last returns up to n samples in chronological (oldest-first) order.
func (r *metricsRing) last(n int) []ServerMetricSample {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n > r.count {
		n = r.count
	}
	if n == 0 {
		return nil
	}
	out := make([]ServerMetricSample, n)
	// head points to the next write slot; the oldest occupied slot is head-count.
	start := (r.head - n + metricsBufferSize*1024) % metricsBufferSize
	for i := 0; i < n; i++ {
		out[i] = r.samples[(start+i)%metricsBufferSize]
	}
	return out
}

// sampleProcess attempts to read CPU and RAM utilisation for a process by PID.
// On non-Linux platforms or when the PID is unavailable it returns (0, 0) silently.
func sampleProcess(pid int) (cpuPct, ramPct float64) {
	if pid <= 0 || runtime.GOOS != "linux" {
		return
	}

	// ── CPU: read /proc/{pid}/stat twice, 100 ms apart ──────────────────────
	readTicks := func() (uint64, error) {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			return 0, err
		}
		fields := strings.Fields(string(data))
		if len(fields) < 15 {
			return 0, fmt.Errorf("unexpected stat format")
		}
		utime, _ := strconv.ParseUint(fields[13], 10, 64)
		stime, _ := strconv.ParseUint(fields[14], 10, 64)
		return utime + stime, nil
	}

	t1, err1 := readTicks()
	wall1 := time.Now()
	time.Sleep(100 * time.Millisecond)
	t2, err2 := readTicks()
	wall2 := time.Now()

	if err1 == nil && err2 == nil {
		// /proc/stat tick rate is usually 100 Hz (USER_HZ = 100).
		const ticksPerSec = 100.0
		elapsedSec := wall2.Sub(wall1).Seconds()
		deltaTicks := float64(t2 - t1)
		if elapsedSec > 0 {
			cpuPct = (deltaTicks / ticksPerSec) / elapsedSec * 100.0
			if cpuPct > 100.0 {
				cpuPct = 100.0
			}
		}
	}

	// ── RAM: VmRSS from /proc/{pid}/status vs MemTotal from /proc/meminfo ──
	statusData, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err == nil {
		var vmRSSKB uint64
		scanner := bufio.NewScanner(strings.NewReader(string(statusData)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					vmRSSKB, _ = strconv.ParseUint(fields[1], 10, 64)
				}
				break
			}
		}

		memData, err2 := os.ReadFile("/proc/meminfo")
		if err2 == nil {
			var memTotalKB uint64
			sc2 := bufio.NewScanner(strings.NewReader(string(memData)))
			for sc2.Scan() {
				line := sc2.Text()
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						memTotalKB, _ = strconv.ParseUint(fields[1], 10, 64)
					}
					break
				}
			}
			if memTotalKB > 0 {
				ramPct = float64(vmRSSKB) / float64(memTotalKB) * 100.0
				if ramPct > 100.0 {
					ramPct = 100.0
				}
			}
		}
	}

	return
}

// dockerSample holds the result of a single docker stats poll.
type dockerSample struct {
	CPUPct  float64
	RAMPct  float64
	NetInB  uint64 // cumulative bytes received since container start
	NetOutB uint64 // cumulative bytes sent since container start
}

// sampleDocker runs `docker stats --no-stream` to get CPU/RAM/network for a container.
// Returns a zero dockerSample silently on any error.
func sampleDocker(containerID string) dockerSample {
	if containerID == "" {
		return dockerSample{}
	}
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return dockerSample{}
	}

	out, err := exec.Command(dockerPath, //nolint:gosec
		"stats", "--no-stream",
		"--format", `{"cpu":"{{.CPUPerc}}","mem":"{{.MemPerc}}","net_in":"{{.NetInput}}","net_out":"{{.NetOutput}}"}`,
		containerID,
	).Output()
	if err != nil {
		return dockerSample{}
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return dockerSample{}
	}

	var parsed struct {
		CPU    string `json:"cpu"`
		Mem    string `json:"mem"`
		NetIn  string `json:"net_in"`
		NetOut string `json:"net_out"`
	}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return dockerSample{}
	}

	parsePct := func(s string) float64 {
		s = strings.TrimSuffix(strings.TrimSpace(s), "%")
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}

	return dockerSample{
		CPUPct:  parsePct(parsed.CPU),
		RAMPct:  parsePct(parsed.Mem),
		NetInB:  parseDockerBytes(parsed.NetIn),
		NetOutB: parseDockerBytes(parsed.NetOut),
	}
}

// parseDockerBytes parses a Docker-formatted byte string like "1.2kB", "3.4MB", "500B", "1.2GB"
// and returns the value in bytes. Returns 0 on parse error.
func parseDockerBytes(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0B" {
		return 0
	}
	units := []struct {
		suffix string
		mult   float64
	}{
		{"GB", 1 << 30}, {"MB", 1 << 20}, {"kB", 1 << 10}, {"B", 1},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			num := strings.TrimSuffix(s, u.suffix)
			v, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
			if err != nil {
				return 0
			}
			return uint64(v * u.mult)
		}
	}
	return 0
}
