package main

import "testing"

func TestStatsOddAndEven(t *testing.T) {
	odd := stats([]float64{5, 1, 3})
	if odd.Median != 3 || odd.Min != 1 || odd.Max != 5 {
		t.Fatalf("odd stats: %+v", odd)
	}
	even := stats([]float64{4, 1, 3, 2})
	if even.Median != 2.5 || even.Min != 1 || even.Max != 4 {
		t.Fatalf("even stats: %+v", even)
	}
}

func TestValidateShapeRejectsChangedCase(t *testing.T) {
	base := runResult{Run: 1, Calls: []callResult{{Sequence: 1, Event: "entry", Case: "a", Actor: "Supplier"}}}
	changed := runResult{Run: 2, Calls: []callResult{{Sequence: 1, Event: "entry", Case: "b", Actor: "Supplier"}}}
	if err := validateShape([]runResult{base, changed}); err == nil {
		t.Fatal("expected shape mismatch")
	}
}
