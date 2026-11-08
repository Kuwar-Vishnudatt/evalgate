// Package judge scores the two RAGAS metrics that are fundamentally
// semantic judgments, not set comparisons: faithfulness (does the
// generated answer only make claims the retrieved context actually
// supports?) and answer relevancy (does the answer actually address the
// question?). Context precision and context recall, by contrast, are
// computed algorithmically against ground-truth context IDs — see
// internal/metrics — because those ARE set-membership questions once you
// have a ground truth to compare against.
package judge

import "context"

// Judge scores one (question, answer, contexts) triple on both semantic
// dimensions. Implementations must be safe for concurrent use.
type Judge interface {
	ScoreFaithfulness(ctx context.Context, question, answer string, contexts []string) (float64, error)
	ScoreAnswerRelevancy(ctx context.Context, question, answer string) (float64, error)
}
