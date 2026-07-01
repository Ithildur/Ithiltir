package alert

import (
	"reflect"
	"testing"
)

func TestSplitSummaryMetrics(t *testing.T) {
	got := splitSummaryMetrics("node.offline\n\ncpu.load1\n ", "fallback")
	want := []string{"node.offline", "cpu.load1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitSummaryMetrics = %#v, want %#v", got, want)
	}
}

func TestSplitSummaryMetricsFallback(t *testing.T) {
	got := splitSummaryMetrics("", "node.offline")
	want := []string{"node.offline"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitSummaryMetrics fallback = %#v, want %#v", got, want)
	}
}
