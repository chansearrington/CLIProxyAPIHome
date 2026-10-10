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
