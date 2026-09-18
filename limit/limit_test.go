package limit

import (
	"reflect"
	"testing"
)

func TestBuildPlanBasic(t *testing.T) {
	plan := BuildPlan([]Entry{
		{InboundId: 1, Port: 443, RateMbps: 50, Remark: "a"},
		{InboundId: 2, Port: 8443, RateMbps: 30, Remark: "b"},
	})

	if plan.Empty() {
		t.Fatal("plan should not be empty")
	}
	if want := []int{30, 50}; !reflect.DeepEqual(plan.Rates, want) {
		t.Fatalf("rates = %v, want %v", plan.Rates, want)
	}
	if plan.ClassOfRate[30] != 10 || plan.ClassOfRate[50] != 11 {
		t.Fatalf("unexpected class mapping: %v", plan.ClassOfRate)
	}

	want := []Rule{
		{Port: 443, RateMbps: 50, ClassId: 11},
		{Port: 8443, RateMbps: 30, ClassId: 10},
	}
	if !reflect.DeepEqual(plan.Rules, want) {
		t.Fatalf("rules = %v, want %v", plan.Rules, want)
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", plan.Warnings)
	}
}

func TestBuildPlanSharesRateClass(t *testing.T) {
	plan := BuildPlan([]Entry{
		{Port: 443, RateMbps: 30},
		{Port: 8443, RateMbps: 30},
	})

	if len(plan.Rates) != 1 {
		t.Fatalf("rates = %v, want a single rate", plan.Rates)
	}
	if len(plan.Rules) != 2 {
		t.Fatalf("rules = %v, want two rules", plan.Rules)
	}
	for _, r := range plan.Rules {
		if r.ClassId != plan.ClassOfRate[30] {
			t.Fatalf("same rate should share one class, got %v", plan.Rules)
		}
	}
}

func TestBuildPlanDuplicatePortKeepsStrictest(t *testing.T) {
	plan := BuildPlan([]Entry{
		{InboundId: 1, Port: 443, RateMbps: 100},
		{InboundId: 2, Port: 443, RateMbps: 20},
	})

	if len(plan.Rules) != 1 {
		t.Fatalf("rules = %v, want a single rule", plan.Rules)
	}
	if plan.Rules[0].RateMbps != 20 {
		t.Fatalf("rate = %d, want the strictest 20", plan.Rules[0].RateMbps)
	}
	if len(plan.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one warning", plan.Warnings)
	}
}

func TestBuildPlanIgnoresInvalidAndUnlimited(t *testing.T) {
	plan := BuildPlan([]Entry{
		{InboundId: 1, Port: 0, RateMbps: 30},
		{InboundId: 2, Port: 443, RateMbps: 0},
		{InboundId: 3, Port: 8443, RateMbps: -1},
	})

	if !plan.Empty() {
		t.Fatalf("plan should be empty, got %v", plan.Rules)
	}
	if len(plan.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one warning about the invalid port", plan.Warnings)
	}
}

func TestSameRatesAndRules(t *testing.T) {
	a := BuildPlan([]Entry{{Port: 443, RateMbps: 30}, {Port: 8443, RateMbps: 50}})
	b := BuildPlan([]Entry{{Port: 8443, RateMbps: 50}, {Port: 443, RateMbps: 30}})
	if !sameRates(a, b) || !sameRules(a, b) {
		t.Fatal("order of entries should not matter")
	}

	c := BuildPlan([]Entry{{Port: 443, RateMbps: 30}, {Port: 8443, RateMbps: 30}})
	if sameRates(a, c) || sameRules(a, c) {
		t.Fatal("different rate sets should not be treated as equal")
	}

	d := BuildPlan([]Entry{{Port: 443, RateMbps: 30}, {Port: 8443, RateMbps: 50}, {Port: 9000, RateMbps: 50}})
	if !sameRates(a, d) {
		t.Fatal("same rate set should be treated as equal")
	}
	if sameRules(a, d) {
		t.Fatal("different port sets should not be treated as equal")
	}
}

func TestDescribe(t *testing.T) {
	plan := BuildPlan([]Entry{{Port: 43667, RateMbps: 30}})
	if got, want := plan.Describe(), "端口 43667 -> 30Mbps"; got != want {
		t.Fatalf("describe = %q, want %q", got, want)
	}
	if got := BuildPlan(nil).Describe(); got != "无" {
		t.Fatalf("describe = %q, want 无", got)
	}
}
