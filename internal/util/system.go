package util

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var StartTime = time.Now()

func GetUptimeFormatted() string {
	uptime := time.Since(StartTime)
	totalSec := int(uptime.Seconds())

	days := totalSec / 86400
	hours := (totalSec % 86400) / 3600
	minutes := (totalSec % 3600) / 60
	seconds := totalSec % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%ds", seconds))

	return strings.Join(parts, " ")
}

func GetProcessRSS() string {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return "N/A"
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseFloat(fields[1], 64)
				if err == nil {
					return fmt.Sprintf("%.2f MB", kb/1024.0)
				}
			}
		}
	}
	return "N/A"
}

func GetHardwareReport() (processor, cpu, cpuLoad, gpu string) {
	numCPU := runtime.NumCPU()
	processor = "Unknown Processor"
	cpuModel := ""
	hardware := ""

	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])

				if key == "model name" && cpuModel == "" {
					cpuModel = val
				} else if key == "Hardware" && hardware == "" {
					hardware = val
				} else if key == "Processor" && processor == "Unknown Processor" {
					processor = val
				}
			}
		}
	}

	maxFreqGHz := 0.0
	for i := 0; i < numCPU; i++ {
		freqPath := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_max_freq", i)
		if fData, err := os.ReadFile(freqPath); err == nil {
			if khz, err := strconv.ParseFloat(strings.TrimSpace(string(fData)), 64); err == nil {
				ghz := khz / 1000000.0
				if ghz > maxFreqGHz {
					maxFreqGHz = ghz
				}
			}
		}
	}

	if cpuModel == "" {
		if out, err := exec.Command("lscpu").Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Model name:") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						cpuModel = strings.TrimSpace(parts[1])
						break
					}
				}
			}
		}
	}

	if hardware != "" {
		processor = hardware
	}
	if strings.Contains(strings.ToUpper(hardware), "KHAJE") {
		processor = "Qualcomm Snapdragon 680 (KHAJE)"
		gpu = "Qualcomm Adreno 610"
	}

	freqStr := ""
	if maxFreqGHz > 0 {
		freqStr = fmt.Sprintf(" @ %.2f GHz", maxFreqGHz)
	}

	if cpuModel != "" {
		cpu = fmt.Sprintf("%d cores (%s%s)", numCPU, cpuModel, freqStr)
	} else {
		cpu = fmt.Sprintf("%d cores%s", numCPU, freqStr)
	}

	if gpu == "" {
		if out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
			g := strings.TrimSpace(string(out))
			if g != "" {
				gpu = strings.Split(g, "\n")[0]
			}
		}
	}
	if gpu == "" {
		if out, err := exec.Command("lspci").Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "vga") || strings.Contains(lower, "3d controller") || strings.Contains(lower, "display") {
					parts := strings.SplitN(line, ":", 3)
					if len(parts) == 3 {
						gpu = strings.TrimSpace(parts[2])
						break
					}
				}
			}
		}
	}
	if gpu == "" {
		if runtime.GOARCH == "amd64" || runtime.GOARCH == "386" {
			gpu = "Integrated Graphics / Standard VGA"
		} else {
			gpu = "Integrated / Software Render"
		}
	}

	cpuLoad = "N/A"
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			if fields[0] != "0.00" || fields[1] != "0.00" || fields[2] != "0.00" {
				cpuLoad = fmt.Sprintf("%s, %s, %s", fields[0], fields[1], fields[2])
			}
		}
	}

	if cpuLoad == "N/A" {
		totalRatio := 0.0
		validCores := 0
		for i := 0; i < numCPU; i++ {
			curP := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i)
			maxP := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_max_freq", i)
			curData, err1 := os.ReadFile(curP)
			maxData, err2 := os.ReadFile(maxP)
			if err1 == nil && err2 == nil {
				cur, _ := strconv.ParseFloat(strings.TrimSpace(string(curData)), 64)
				max, _ := strconv.ParseFloat(strings.TrimSpace(string(maxData)), 64)
				if max > 0 {
					totalRatio += (cur / max)
					validCores++
				}
			}
		}
		if validCores > 0 {
			avg := (totalRatio / float64(validCores)) * 100.0
			cpuLoad = fmt.Sprintf("%.1f%%", avg)
		}
	}

	return processor, cpu, cpuLoad, gpu
}
