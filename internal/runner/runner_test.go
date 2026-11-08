package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/Kuwar-Vishnudatt/evalgate/internal/golden"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/judge"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/target"
)

type stubTarget struct {
	responses map[string]target.Response
	errs      map[string]error
}

func (s stubTarget) Generate(_ context.Context, question string) (target.Response, error) {
	if err, ok := s.errs[question]; ok {
		return target.Response{}, err
	}
	return s.responses[question], nil
}

func TestRunProducesPerCaseAndAggregateScores(t *testing.T) {
	cases := []golden.Case{
		{ID: "c1", Question: "warranty length", RelevantContextIDs: []string{"doc1"}},
	}
	st := stubTarget{responses: map[string]target.Response{
		"warranty length": {
			Answer:   "The warranty lasts two years.",
			Contexts: []target.Context{{ID: "doc1", Text: "Our product warranty lasts two years from purchase."}},
		},
	}}

	res, err := Run(context.Background(), cases, st, judge.NewFake())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cases) != 1 {
		t.Fatalf("expected 1 case result, got %d", len(res.Cases))
	}
	c := res.Cases[0]
	if c.Err != "" {
		t.Fatalf("expected no error, got %q", c.Err)
	}
	if c.ContextRecall != 1.0 || c.ContextPrecision != 1.0 {
		t.Fatalf("expected perfect context precision/recall (doc1 retrieved and relevant), got %+v", c)
	}
	if c.Faithfulness < 0.9 {
		t.Fatalf("expected high faithfulness for a well-supported answer, got %v", c.Faithfulness)
	}

	if res.Aggregate.NumCases != 1 || res.Aggregate.NumErrors != 0 {
		t.Fatalf("unexpected aggregate counts: %+v", res.Aggregate)
	}
}

func TestRunRecordsTargetErrorsSeparatelyFromScores(t *testing.T) {
	cases := []golden.Case{
		{ID: "c1", Question: "broken question"},
		{ID: "c2", Question: "working question"},
	}
	st := stubTarget{
		errs:      map[string]error{"broken question": errors.New("target crashed")},
		responses: map[string]target.Response{"working question": {Answer: "fine", Contexts: nil}},
	}

	res, err := Run(context.Background(), cases, st, judge.NewFake())
	if err != nil {
		t.Fatal(err)
	}
	if res.Aggregate.NumCases != 2 || res.Aggregate.NumErrors != 1 {
		t.Fatalf("expected 2 total cases and 1 error, got %+v", res.Aggregate)
	}

	var errored, ok bool
	for _, c := range res.Cases {
		if c.CaseID == "c1" && c.Err != "" {
			errored = true
		}
		if c.CaseID == "c2" && c.Err == "" {
			ok = true
		}
	}
	if !errored || !ok {
		t.Fatalf("expected c1 to carry an error and c2 to succeed, got %+v", res.Cases)
	}
}

func TestAggregateExcludesErroredCasesFromMean(t *testing.T) {
	// A crashing case must not silently drag the aggregate toward zero —
	// it should be excluded from the mean entirely, with NumErrors
	// reporting it separately.
	cases := []golden.Case{
		{ID: "c1", Question: "broken"},
		{ID: "c2", Question: "good"},
	}
	st := stubTarget{
		errs: map[string]error{"broken": errors.New("boom")},
		responses: map[string]target.Response{
			"good": {Answer: "the warranty is two years", Contexts: []target.Context{{ID: "d1", Text: "the warranty is two years"}}},
		},
	}

	res, err := Run(context.Background(), cases, st, judge.NewFake())
	if err != nil {
		t.Fatal(err)
	}
	if res.Aggregate.Faithfulness <= 0 {
		t.Fatalf("expected the aggregate to reflect only the successful case, got %+v", res.Aggregate)
	}
}
