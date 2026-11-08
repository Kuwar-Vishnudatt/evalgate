// Package runner ties golden cases, a target, a judge, and the metrics
// package into one Run() call that produces per-case and aggregate scores
// — the thing a CI job actually invokes.
package runner

import (
	"context"
	"fmt"

	"github.com/Kuwar-Vishnudatt/evalgate/internal/golden"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/judge"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/metrics"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/target"
)

type CaseResult struct {
	CaseID           string  `json:"case_id"`
	Faithfulness     float64 `json:"faithfulness"`
	AnswerRelevancy  float64 `json:"answer_relevancy"`
	ContextPrecision float64 `json:"context_precision"`
	ContextRecall    float64 `json:"context_recall"`
	Answer           string  `json:"answer"`
	Err              string  `json:"error,omitempty"`
}

// Aggregate is the mean of each metric across every case that didn't
// error. A case-level error is excluded from the mean (not treated as a
// zero score) and surfaced separately in NumErrors — conflating "the
// target crashed on this input" with "the target answered this input
// badly" would make a flaky target's score misleadingly bad in a way a
// human reading just the aggregate couldn't distinguish from genuine
// quality regression.
type Aggregate struct {
	Faithfulness     float64 `json:"faithfulness"`
	AnswerRelevancy  float64 `json:"answer_relevancy"`
	ContextPrecision float64 `json:"context_precision"`
	ContextRecall    float64 `json:"context_recall"`
	NumCases         int     `json:"num_cases"`
	NumErrors        int     `json:"num_errors"`
}

type Result struct {
	Cases     []CaseResult `json:"cases"`
	Aggregate Aggregate    `json:"aggregate"`
}

// Run evaluates every case in cases against t, scoring each with j and
// the algorithmic context metrics.
func Run(ctx context.Context, cases []golden.Case, t target.Target, j judge.Judge) (Result, error) {
	var results []CaseResult

	for _, c := range cases {
		resp, err := t.Generate(ctx, c.Question)
		if err != nil {
			results = append(results, CaseResult{CaseID: c.ID, Err: err.Error()})
			continue
		}

		contextTexts := make([]string, len(resp.Contexts))
		contextIDs := make([]string, len(resp.Contexts))
		for i, rc := range resp.Contexts {
			contextTexts[i] = rc.Text
			contextIDs[i] = rc.ID
		}

		faithfulness, err := j.ScoreFaithfulness(ctx, c.Question, resp.Answer, contextTexts)
		if err != nil {
			results = append(results, CaseResult{CaseID: c.ID, Err: fmt.Sprintf("faithfulness: %v", err)})
			continue
		}
		relevancy, err := j.ScoreAnswerRelevancy(ctx, c.Question, resp.Answer)
		if err != nil {
			results = append(results, CaseResult{CaseID: c.ID, Err: fmt.Sprintf("answer relevancy: %v", err)})
			continue
		}

		results = append(results, CaseResult{
			CaseID:           c.ID,
			Faithfulness:     faithfulness,
			AnswerRelevancy:  relevancy,
			ContextPrecision: metrics.ContextPrecision(contextIDs, c.RelevantContextIDs),
			ContextRecall:    metrics.ContextRecall(contextIDs, c.RelevantContextIDs),
			Answer:           resp.Answer,
		})
	}

	return Result{Cases: results, Aggregate: aggregate(results)}, nil
}

func aggregate(results []CaseResult) Aggregate {
	var agg Aggregate
	ok := 0
	for _, r := range results {
		agg.NumCases++
		if r.Err != "" {
			agg.NumErrors++
			continue
		}
		agg.Faithfulness += r.Faithfulness
		agg.AnswerRelevancy += r.AnswerRelevancy
		agg.ContextPrecision += r.ContextPrecision
		agg.ContextRecall += r.ContextRecall
		ok++
	}
	if ok > 0 {
		agg.Faithfulness /= float64(ok)
		agg.AnswerRelevancy /= float64(ok)
		agg.ContextPrecision /= float64(ok)
		agg.ContextRecall /= float64(ok)
	}
	return agg
}
