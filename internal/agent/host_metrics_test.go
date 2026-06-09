package agent

import (
	"context"
	"strings"
	"testing"
)

func TestCollectHostGauges_HasMemoryAndCPUUtilization(t *testing.T) {
	ctx := context.Background()
	got := CollectHostGauges(ctx)

	if _, ok := got["TotalMemory"]; !ok {
		t.Fatalf("missing TotalMemory")
	}
	if got["TotalMemory"] <= 0 {
		t.Fatalf("TotalMemory = %f, want > 0", got["TotalMemory"])
	}
	if _, ok := got["FreeMemory"]; !ok {
		t.Fatalf("missing FreeMemory")
	}

	var cpuKeys int
	for k := range got {
		if strings.HasPrefix(k, "CPUutilization") {
			cpuKeys++
		}
	}
	if cpuKeys == 0 {
		t.Fatalf("expected at least one CPUutilizationN gauge")
	}
}
