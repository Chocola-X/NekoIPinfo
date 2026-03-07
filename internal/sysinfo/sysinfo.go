package sysinfo

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type Info struct {
	TotalMemMB   uint64
	AvailMemMB   uint64
	UsedMemMB    uint64
	CPUCores     int
	CPUThreads   int
	CPUModelName string
	GoAllocMB    float64
	GoSysMB      float64
	NumGC        uint32
	NumGoroutine int
}

func Collect() Info {
	var si Info
	si.CPUThreads = runtime.NumCPU()
	si.CPUCores = si.CPUThreads

	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		si.CPUModelName = infos[0].ModelName
		cores := 0
		for _, c := range infos {
			cores += int(c.Cores)
		}
		if cores > 0 {
			si.CPUCores = cores
		}
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		si.TotalMemMB = vm.Total / (1024 * 1024)
		si.AvailMemMB = vm.Available / (1024 * 1024)
		si.UsedMemMB = vm.Used / (1024 * 1024)
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	si.GoAllocMB = float64(m.Alloc) / (1024 * 1024)
	si.GoSysMB = float64(m.Sys) / (1024 * 1024)
	si.NumGC = m.NumGC
	si.NumGoroutine = runtime.NumGoroutine()

	return si
}

func AvailableMemoryMB() uint64 {
	if vm, err := mem.VirtualMemory(); err == nil {
		return vm.Available / (1024 * 1024)
	}
	return 0
}

func TotalMemoryMB() uint64 {
	if vm, err := mem.VirtualMemory(); err == nil {
		return vm.Total / (1024 * 1024)
	}
	return 0
}

func DatabaseSizeMB(dbPath string) float64 {
	var totalSize int64
	filepath.Walk(dbPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return float64(totalSize) / (1024 * 1024)
}