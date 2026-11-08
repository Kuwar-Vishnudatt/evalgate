package gate

import (
	"testing"

	"github.com/Kuwar-Vishnudatt/evalgate/internal/runner"
)

func TestPassesWhenAboveThresholdsNoBaseline(t *testing.T) {
	cand := runner.Aggregate{Faithfulness: 0.9, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	th := Thresholds{MinFaithfulness: 0.7, MinAnswerRelevancy: 0.7, MinContextPrecision: 0.7, MinContextRecall: 0.7}

	d := Evaluate(cand, nil, th)
	if d.Blocked {
		t.Fatalf("expected a pass when every metric clears its threshold, got %+v", d)
	}
}

func TestFailsWhenBelowAbsoluteThreshold(t *testing.T) {
	cand := runner.Aggregate{Faithfulness: 0.5, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	th := Thresholds{MinFaithfulness: 0.7, MinAnswerRelevancy: 0.7, MinContextPrecision: 0.7, MinContextRecall: 0.7}

	d := Evaluate(cand, nil, th)
	if !d.Blocked {
		t.Fatal("expected a block when faithfulness is below its absolute threshold")
	}
}

func TestFailsOnRegressionEvenAboveThreshold(t *testing.T) {
	// 0.80 clears a 0.7 floor, but dropped from a 0.95 baseline by more
	// than the allowed regression tolerance — must still block.
	cand := runner.Aggregate{Faithfulness: 0.80, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	base := runner.Aggregate{Faithfulness: 0.95, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	th := Thresholds{MinFaithfulness: 0.7, MinAnswerRelevancy: 0.7, MinContextPrecision: 0.7, MinContextRecall: 0.7, MaxRegression: 0.05}

	d := Evaluate(cand, &base, th)
	if !d.Blocked {
		t.Fatal("expected a block for a regression beyond tolerance, even though the absolute threshold is still cleared")
	}
	for _, v := range d.Verdicts {
		if v.Name == "faithfulness" && !v.Regressed {
			t.Fatal("expected the faithfulness verdict to be flagged as regressed")
		}
	}
}

func TestSmallDropWithinToleranceDoesNotBlock(t *testing.T) {
	cand := runner.Aggregate{Faithfulness: 0.92, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	base := runner.Aggregate{Faithfulness: 0.95, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	th := Thresholds{MinFaithfulness: 0.7, MinAnswerRelevancy: 0.7, MinContextPrecision: 0.7, MinContextRecall: 0.7, MaxRegression: 0.05}

	d := Evaluate(cand, &base, th)
	if d.Blocked {
		t.Fatalf("expected a 0.03 drop to be within a 0.05 tolerance, got %+v", d)
	}
}

func TestImprovementNeverBlocks(t *testing.T) {
	cand := runner.Aggregate{Faithfulness: 0.99, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	base := runner.Aggregate{Faithfulness: 0.80, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	th := Thresholds{MinFaithfulness: 0.7, MinAnswerRelevancy: 0.7, MinContextPrecision: 0.7, MinContextRecall: 0.7, MaxRegression: 0.05}

	d := Evaluate(cand, &base, th)
	if d.Blocked {
		t.Fatal("an improvement over baseline must never be blocked")
	}
}

func TestRegressionToleranceBoundaryIsExclusive(t *testing.T) {
	// Exactly at the tolerance boundary (drop == MaxRegression) should NOT
	// be flagged as a regression — only a drop STRICTLY GREATER than the
	// tolerance should block, since the tolerance is the amount of
	// movement explicitly declared acceptable.
	cand := runner.Aggregate{Faithfulness: 0.90, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	base := runner.Aggregate{Faithfulness: 0.95, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	th := Thresholds{MinFaithfulness: 0.7, MinAnswerRelevancy: 0.7, MinContextPrecision: 0.7, MinContextRecall: 0.7, MaxRegression: 0.05}

	d := Evaluate(cand, &base, th)
	if d.Blocked {
		t.Fatalf("expected a drop exactly at the tolerance boundary to be allowed, got %+v", d)
	}
}

func TestNoBaselineOnlyChecksAbsoluteThreshold(t *testing.T) {
	cand := runner.Aggregate{Faithfulness: 0.5, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	th := Thresholds{MinFaithfulness: 0.3, MinAnswerRelevancy: 0.7, MinContextPrecision: 0.7, MinContextRecall: 0.7}

	d := Evaluate(cand, nil, th)
	if d.Blocked {
		t.Fatal("with no baseline, a metric above its absolute threshold must pass regardless")
	}
	for _, v := range d.Verdicts {
		if v.HasBaseline {
			t.Fatalf("expected no verdict to reference a baseline when none was given, got %+v", v)
		}
	}
}

func TestReportIncludesBlockedSummary(t *testing.T) {
	cand := runner.Aggregate{Faithfulness: 0.5, AnswerRelevancy: 0.9, ContextPrecision: 0.9, ContextRecall: 0.9}
	th := Thresholds{MinFaithfulness: 0.7, MinAnswerRelevancy: 0.7, MinContextPrecision: 0.7, MinContextRecall: 0.7}
	d := Evaluate(cand, nil, th)
	report := Report(d)
	if !contains(report, "BLOCKED") || !contains(report, "BELOW THRESHOLD") {
		t.Fatalf("expected the report to mention the block and the specific failing metric, got:\n%s", report)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
