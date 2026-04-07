package db

import "testing"

func TestIntroEndFromPair(t *testing.T) {
	n := 200
	fpA := make([]uint32, n)
	fpB := make([]uint32, n)
	for i := range fpA {
		fpA[i] = 0x11111111
		fpB[i] = 0x11111111
	}
	for i := 80; i < n; i++ {
		fpB[i] = 0xeeeeeeee
	}
	end, ok := introEndFromPair(fpA, fpB, 120)
	if !ok {
		t.Fatal("expected match")
	}
	if end < 40 || end > 55 {
		t.Fatalf("end = %v, want ~48 (80/200*120)", end)
	}
}

func TestMedianFloat(t *testing.T) {
	m, ok := medianFloat([]float64{3, 1, 2})
	if !ok || m != 2 {
		t.Fatalf("median = %v ok=%v", m, ok)
	}
}
