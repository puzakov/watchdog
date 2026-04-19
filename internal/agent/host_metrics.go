package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// CollectHostGauges reads host memory and per-CPU utilization (gopsutil).
// Gauge names: TotalMemory, FreeMemory, CPUutilization1..CPUutilizationN.
func CollectHostGauges(ctx context.Context) map[string]float64 {
	out := make(map[string]float64)

	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err == nil && vm != nil {
		out["TotalMemory"] = float64(vm.Total)
		out["FreeMemory"] = float64(vm.Free)
	}

	interval := 200 * time.Millisecond
	percents, err := cpu.PercentWithContext(ctx, interval, true)
	if err != nil || len(percents) == 0 {
		return out
	}
	for i, p := range percents {
		out[fmt.Sprintf("CPUutilization%d", i+1)] = p
	}
	return out
}
