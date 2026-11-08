package example

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuwar-Vishnudatt/evalgate/internal/gate"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/golden"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/judge"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/runner"
)

func loadGoldenSet(t *testing.T) []golden.Case {
	t.Helper()
	root := findRepoRoot(t)
	cases, err := golden.Load(filepath.Join(root, "testdata", "golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	return cases
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func TestGoodTargetScoresHighAcrossAllMetrics(t *testing.T) {
	cases := loadGoldenSet(t)
	res, err := runner.Run(context.Background(), cases, GoodTarget{Docs: FAQDocs()}, judge.NewFake())
	if err != nil {
		t.Fatal(err)
	}
	if res.Aggregate.Faithfulness < 0.8 {
		t.Fatalf("expected high faithfulness for the good target, got %v", res.Aggregate.Faithfulness)
	}
	if res.Aggregate.ContextRecall < 0.9 {
		t.Fatalf("expected high context recall for the good target, got %v", res.Aggregate.ContextRecall)
	}
}

func TestDegradedTargetHasLowerFaithfulnessThanGood(t *testing.T) {
	cases := loadGoldenSet(t)
	goodRes, err := runner.Run(context.Background(), cases, GoodTarget{Docs: FAQDocs()}, judge.NewFake())
	if err != nil {
		t.Fatal(err)
	}
	degradedRes, err := runner.Run(context.Background(), cases, DegradedTarget{Docs: FAQDocs()}, judge.NewFake())
	if err != nil {
		t.Fatal(err)
	}

	if degradedRes.Aggregate.Faithfulness >= goodRes.Aggregate.Faithfulness {
		t.Fatalf("expected the degraded target's fabricated claim to lower faithfulness: good=%v degraded=%v",
			goodRes.Aggregate.Faithfulness, degradedRes.Aggregate.Faithfulness)
	}
}

// TestCIGateBlocksTheDegradedTargetButPassesTheGoodOne is the capstone
// proof for the whole project: wiring the real runner, real Fake judge,
// real metrics, and the real gate decision together, and confirming the
// gate produces the opposite verdict for the good and degraded targets —
// not just "the numbers differ," but "a CI job using this gate would
// actually pass one and block the other."
func TestCIGateBlocksTheDegradedTargetButPassesTheGoodOne(t *testing.T) {
	cases := loadGoldenSet(t)
	th := gate.Thresholds{MinFaithfulness: 0.75, MinAnswerRelevancy: 0.1, MinContextPrecision: 0.5, MinContextRecall: 0.5}

	goodRes, err := runner.Run(context.Background(), cases, GoodTarget{Docs: FAQDocs()}, judge.NewFake())
	if err != nil {
		t.Fatal(err)
	}
	goodDecision := gate.Evaluate(goodRes.Aggregate, nil, th)
	if goodDecision.Blocked {
		t.Fatalf("expected the good target to PASS the gate, got blocked: %s", gate.Report(goodDecision))
	}

	degradedRes, err := runner.Run(context.Background(), cases, DegradedTarget{Docs: FAQDocs()}, judge.NewFake())
	if err != nil {
		t.Fatal(err)
	}
	degradedDecision := gate.Evaluate(degradedRes.Aggregate, &goodRes.Aggregate, th)
	if !degradedDecision.Blocked {
		t.Fatalf("expected the degraded target to be BLOCKED by the gate, got: %s", gate.Report(degradedDecision))
	}
}
