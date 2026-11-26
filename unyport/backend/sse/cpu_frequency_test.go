package sse

import "testing"

func TestComputeEffectiveFreqsAveragesAllCores(t *testing.T) {
	t1 := map[string]cpuTimes{
		"cpu0": {idle: 0, total: 100},
		"cpu1": {idle: 0, total: 100},
	}
	t2 := map[string]cpuTimes{
		"cpu0": {idle: 50, total: 200},
		"cpu1": {idle: 75, total: 200},
	}

	average, cores := computeEffectiveFreqs(t1, t2, 3500)
	if average != 1312 {
		t.Fatalf("average = %d MHz, want 1312 MHz", average)
	}
	if len(cores) != 2 || cores[0] != 1750 || cores[1] != 875 {
		t.Fatalf("cores = %v, want [1750 875]", cores)
	}
}
