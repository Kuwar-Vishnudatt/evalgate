// Package gate is the CI decision layer: given a candidate's aggregate
// scores (and, optionally, a baseline's), decide pass or fail. This is
// what makes evalgate a reusable CI tool rather than a one-off eval
// script — the script just prints numbers; this package is what turns
// those numbers into an exit code a PR check can gate on.
package gate

import (
	"fmt"

	"github.com/Kuwar-Vishnudatt/evalgate/internal/runner"
)

// Thresholds is the configured policy: an absolute minimum per metric,
// and (if a baseline is supplied) the maximum tolerated drop versus it.
type Thresholds struct {
	MinFaithfulness     float64 `yaml:"min_faithfulness"`
	MinAnswerRelevancy  float64 `yaml:"min_answer_relevancy"`
	MinContextPrecision float64 `yaml:"min_context_precision"`
	MinContextRecall    float64 `yaml:"min_context_recall"`

	// MaxRegression is the maximum allowed drop (candidate - baseline,
	// more negative is worse) for any metric before it's a failure, even
	// if the candidate is still above its absolute minimum. A metric can
	// be well above its floor and still represent a real regression worth
	// blocking — e.g. faithfulness dropping from 0.95 to 0.80 is a
	// meaningful quality regression even if 0.80 clears a 0.7 floor.
	MaxRegression float64 `yaml:"max_regression"`
}

type MetricVerdict struct {
	Name           string  `json:"name"`
	Candidate      float64 `json:"candidate"`
	Baseline       float64 `json:"baseline,omitempty"`
	HasBaseline    bool    `json:"has_baseline"`
	BelowThreshold bool    `json:"below_threshold"`
	Regressed      bool    `json:"regressed"`
}

type Decision struct {
	Verdicts []MetricVerdict `json:"verdicts"`
	Blocked  bool            `json:"blocked"`
}

// Evaluate applies Thresholds to a candidate run, and — if baseline is
// non-nil — also checks for regression against it.
func Evaluate(candidate runner.Aggregate, baseline *runner.Aggregate, th Thresholds) Decision {
	specs := []struct {
		name string
		val  float64
		min  float64
	}{
		{"faithfulness", candidate.Faithfulness, th.MinFaithfulness},
		{"answer_relevancy", candidate.AnswerRelevancy, th.MinAnswerRelevancy},
		{"context_precision", candidate.ContextPrecision, th.MinContextPrecision},
		{"context_recall", candidate.ContextRecall, th.MinContextRecall},
	}

	var baselineVals map[string]float64
	if baseline != nil {
		baselineVals = map[string]float64{
			"faithfulness":      baseline.Faithfulness,
			"answer_relevancy":  baseline.AnswerRelevancy,
			"context_precision": baseline.ContextPrecision,
			"context_recall":    baseline.ContextRecall,
		}
	}

	var decision Decision
	for _, s := range specs {
		v := MetricVerdict{Name: s.name, Candidate: s.val}
		if s.val < s.min {
			v.BelowThreshold = true
		}
		if baselineVals != nil {
			base := baselineVals[s.name]
			v.Baseline = base
			v.HasBaseline = true
			if s.val-base < -th.MaxRegression {
				v.Regressed = true
			}
		}
		if v.BelowThreshold || v.Regressed {
			decision.Blocked = true
		}
		decision.Verdicts = append(decision.Verdicts, v)
	}
	return decision
}

// Report formats a Decision as CI-log-friendly text.
func Report(d Decision) string {
	s := ""
	for _, v := range d.Verdicts {
		status := "OK"
		if v.BelowThreshold {
			status = "BELOW THRESHOLD"
		} else if v.Regressed {
			status = "REGRESSED"
		}
		if v.HasBaseline {
			s += fmt.Sprintf("  %-18s candidate=%.3f baseline=%.3f  [%s]\n", v.Name, v.Candidate, v.Baseline, status)
		} else {
			s += fmt.Sprintf("  %-18s candidate=%.3f  [%s]\n", v.Name, v.Candidate, status)
		}
	}
	if d.Blocked {
		s += "\nBLOCKED: one or more metrics failed their threshold or regressed beyond tolerance.\n"
	} else {
		s += "\nOK: all metrics within threshold and regression tolerance.\n"
	}
	return s
}
