package quota

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestClaudeUsageUtilizationIsAlwaysPercentage(t *testing.T) {
	for _, usedPercent := range []float64{0, 0.5, 1, 2, 50, 100} {
		t.Run(fmt.Sprint(usedPercent), func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"five_hour":{"utilization":%g},"seven_day":{"utilization":%g},"extra_usage":{"is_enabled":true,"utilization":%g}}`, usedPercent, usedPercent, usedPercent))
			windows, errParse := parseClaudeUsageWindows(body, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))
			if errParse != nil {
				t.Fatal(errParse)
			}
			if len(windows) != 3 {
				t.Fatalf("got %d windows, want 3", len(windows))
			}
			for _, window := range windows {
				wantUsed := usedPercent / 100
				if window.UsedRatio == nil || math.Abs(*window.UsedRatio-wantUsed) > 1e-10 {
					t.Errorf("%s used ratio = %v, want %g", window.ID, window.UsedRatio, wantUsed)
				}
				if window.RemainingRatio == nil || math.Abs(*window.RemainingRatio-(1-wantUsed)) > 1e-10 {
					t.Errorf("%s remaining ratio = %v, want %g", window.ID, window.RemainingRatio, 1-wantUsed)
				}
				if usedPercent <= 2 && window.Status != "healthy" {
					t.Errorf("%s status = %q for %g%% used, want healthy", window.ID, window.Status, usedPercent)
				}
				if usedPercent == 100 && window.Status != "exhausted" {
					t.Errorf("%s status = %q for 100%% used, want exhausted", window.ID, window.Status)
				}
			}
		})
	}
}

func TestClaudeProfilePlans(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		plan    string
		premium bool
		invalid bool
	}{
		{name: "max20", body: `{"organization":{"organization_type":"claude_max","rate_limit_tier":"default_claude_max_20x"}}`, plan: "Max 20x", premium: true},
		{name: "max5", body: `{"organization":{"organization_type":"claude_max","rate_limit_tier":"default_claude_max_5x"}}`, plan: "Max 5x", premium: true},
		{name: "max unknown multiplier", body: `{"organization":{"organization_type":"claude_max","rate_limit_tier":"future_max_tier"}}`, plan: "Max", premium: true},
		{name: "account max flag", body: `{"account":{"has_claude_max":true}}`, plan: "Max", premium: true},
		{name: "enterprise precedes individual flags", body: `{"organization":{"organization_type":"claude_enterprise","rate_limit_tier":"default_claude_max_20x"},"account":{"has_claude_max":true}}`, plan: "Enterprise"},
		{name: "team", body: `{"organization":{"organization_type":"claude_team"}}`, plan: "Team"},
		{name: "pro", body: `{"organization":{"organization_type":"claude_pro"}}`, plan: "Pro"},
		{name: "account pro flag", body: `{"account":{"has_claude_pro":true}}`, plan: "Pro"},
		{name: "free", body: `{"organization":{"organization_type":"claude_free"}}`, plan: "Free"},
		{name: "unknown is not inferred", body: `{"organization":{"organization_type":"future_plan"},"account":{}}`},
		{name: "empty account is valid but unknown", body: `{"account":{}}`},
		{name: "missing profile", body: `{}`, invalid: true},
		{name: "null profile", body: `{"account":null,"organization":null}`, invalid: true},
		{name: "malformed", body: `{`, invalid: true},
		{name: "invalid field type", body: `{"organization":{"rate_limit_tier":20}}`, invalid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, errParse := parseClaudeProfilePlan([]byte(test.body))
			if (errParse != nil) != test.invalid {
				t.Fatalf("parse error = %v, invalid = %v", errParse, test.invalid)
			}
			if test.plan == "" {
				if plan != nil {
					t.Fatalf("unknown profile produced a plan: %+v", plan)
				}
				return
			}
			if plan == nil || plan.Name != test.plan || plan.Premium != test.premium {
				t.Fatalf("plan = %+v, want %s premium=%v", plan, test.plan, test.premium)
			}
		})
	}
}
