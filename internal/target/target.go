// Package target defines the RAG application under evaluation: given a
// question, it retrieves context and generates an answer. evalgate is
// deliberately agnostic to how a real target does either — it only
// consumes the (contexts, answer) pair, the same boundary RAGAS-style
// evaluation draws in practice.
package target

import "context"

// Context is one retrieved passage.
type Context struct {
	ID   string
	Text string
}

// Response is what a target produces for one question.
type Response struct {
	Answer   string
	Contexts []Context
}

type Target interface {
	Generate(ctx context.Context, question string) (Response, error)
}
