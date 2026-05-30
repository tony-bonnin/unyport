package sse

import "testing"

func TestComputeXenCPUPct(t *testing.T) {
	doms := []XenDomain{
		{DomID: 0, VCPUs: 2, CPUSec: 15},
		{DomID: 7, VCPUs: 1, CPUSec: 103},
	}
	prev := map[int]float64{0: 10, 7: 100}

	cur := computeXenCPUPct(doms, prev, 5)

	if got, want := doms[0].CPUPct, 100.0; got != want {
		t.Fatalf("dom0 cpu pct = %v, want %v", got, want)
	}
	if got, want := doms[1].CPUPct, 60.0; got != want {
		t.Fatalf("dom7 cpu pct = %v, want %v", got, want)
	}
	if got, want := cur[0], 15.0; got != want {
		t.Fatalf("cur dom0 cpu sec = %v, want %v", got, want)
	}
}

func TestComputeXenCPUPctCapsAtVCPUs(t *testing.T) {
	doms := []XenDomain{{DomID: 1, VCPUs: 1, CPUSec: 20}}
	computeXenCPUPct(doms, map[int]float64{1: 0}, 1)

	if got, want := doms[0].CPUPct, 100.0; got != want {
		t.Fatalf("cpu pct cap = %v, want %v", got, want)
	}
}
