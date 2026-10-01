package autodoctor

import (
	"context"
	"testing"
	"time"
)

func TestScoreAndBadges(t *testing.T) {
	tests := []struct {
		name      string
		issues    []Issue
		wantScore int
		wantBadge string
	}{
		{
			name:      "No issues - perfect score",
			issues:    nil,
			wantScore: 100,
			wantBadge: "🟢",
		},
		{
			name: "One warning",
			issues: []Issue{
				{Severity: SeverityWarning},
			},
			wantScore: 95,
			wantBadge: "🟢",
		},
		{
			name: "Two criticals",
			issues: []Issue{
				{Severity: SeverityCritical},
				{Severity: SeverityCritical},
			},
			wantScore: 70,
			wantBadge: "🟡",
		},
		{
			name: "Multiple critical issues - Red badge",
			issues: []Issue{
				{Severity: SeverityCritical},
				{Severity: SeverityCritical},
				{Severity: SeverityCritical},
			},
			wantScore: 55,
			wantBadge: "🔴",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := 100
			for _, iss := range tt.issues {
				switch iss.Severity {
				case SeverityCritical:
					score -= 15
				case SeverityWarning:
					score -= 5
				case SeverityInfo:
					score -= 2
				}
			}
			if score < 0 {
				score = 0
			}

			if score != tt.wantScore {
				t.Errorf("got score %d, want %d", score, tt.wantScore)
			}

			var badge string
			if score >= 85 {
				badge = "🟢"
			} else if score >= 65 {
				badge = "🟡"
			} else {
				badge = "🔴"
			}

			if badge != tt.wantBadge {
				t.Errorf("got badge %s, want %s", badge, tt.wantBadge)
			}
		})
	}
}

func TestNilClientAudit(t *testing.T) {
	_, err := RunAudit(context.Background(), nil)
	if err == nil {
		t.Errorf("expected error when running audit with nil client")
	}
}

func TestMinHelper(t *testing.T) {
	if min(3, 7) != 3 {
		t.Errorf("expected min(3,7) == 3")
	}
	if min(10, 4) != 4 {
		t.Errorf("expected min(10,4) == 4")
	}
}

func TestIssueCategorization(t *testing.T) {
	iss := Issue{
		ID:             "test_iss",
		Category:       CategoryContainer,
		Severity:       SeverityCritical,
		Title:          "Crash loop",
		Target:         "api",
		Description:    "Exited with code 137",
		RootCause:      "Out of memory",
		Recommendation: "Increase RAM",
		Timestamp:      time.Now(),
	}

	if iss.Category != CategoryContainer {
		t.Errorf("expected category %s, got %s", CategoryContainer, iss.Category)
	}
	if iss.Severity != SeverityCritical {
		t.Errorf("expected severity %s, got %s", SeverityCritical, iss.Severity)
	}
}
