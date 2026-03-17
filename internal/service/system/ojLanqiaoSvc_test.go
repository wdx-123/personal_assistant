package system

import (
	"testing"

	lq "personal_assistant/internal/infrastructure/lanqiao"
)

func TestNormalizeLanqiaoPhone(t *testing.T) {
	t.Parallel()

	got, err := normalizeLanqiaoPhone("+86 138-0000-0000")
	if err != nil {
		t.Fatalf("normalizeLanqiaoPhone() error = %v", err)
	}
	if got != "13800000000" {
		t.Fatalf("normalizeLanqiaoPhone() = %q, want %q", got, "13800000000")
	}

	if _, err := normalizeLanqiaoPhone("abc"); err == nil {
		t.Fatal("normalizeLanqiaoPhone() error = nil, want invalid input error")
	}
}

func TestExtractLanqiaoSubmitStatsUsesStatsPayload(t *testing.T) {
	t.Parallel()

	out := &lq.SolveStatsResponse{}
	out.Data.Stats = &lq.SolveStatsStats{
		TotalPassed: 5,
		TotalFailed: 2,
	}
	out.Data.Problems = []lq.SolveStatsProblem{
		{ProblemID: 1, IsPassed: true},
		{ProblemID: 2, IsPassed: true},
		{ProblemID: 3, IsPassed: true},
	}

	successCount, failedCount := extractLanqiaoSubmitStats(out)
	if successCount != 5 || failedCount != 2 {
		t.Fatalf("extractLanqiaoSubmitStats() = (%d,%d), want (5,2)", successCount, failedCount)
	}
}
