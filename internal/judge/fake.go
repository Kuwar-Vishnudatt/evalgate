package judge

import (
	"context"
	"regexp"
	"strings"
)

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

// stopWords are grammatical function words filtered out before any
// word-overlap comparison. Without this, a short question like "How long
// is the warranty?" and a long, fully-supported answer share mostly
// "the"/"is"/"a" rather than the words that actually carry meaning
// ("warranty"), which dilutes every heuristic that follows into noise.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "of": true, "to": true, "in": true, "on": true,
	"at": true, "for": true, "with": true, "by": true, "this": true, "that": true, "these": true,
	"those": true, "it": true, "its": true, "and": true, "or": true, "but": true, "if": true,
	"then": true, "so": true, "as": true, "from": true, "than": true,
}

func words(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
		if !stopWords[w] {
			m[w] = true
		}
	}
	return m
}

// containment is the fraction of a's words that also appear in b — a
// recall-style measure, not Jaccard's symmetric intersection-over-union.
// This matters specifically for answer relevancy: a genuinely relevant
// answer is almost always LONGER and more detailed than the question that
// prompted it, so Jaccard's union-based denominator would penalize every
// well-supported, verbose-but-relevant answer for containing additional
// (true, useful) words the question didn't use — exactly the wrong
// signal for "does this answer address the question."
func containment(a, b map[string]bool) float64 {
	if len(a) == 0 {
		return 1
	}
	hit := 0
	for w := range a {
		if b[w] {
			hit++
		}
	}
	return float64(hit) / float64(len(a))
}

// splitClaims deterministically decomposes an answer into sentence-level
// "claims" — the same granularity real RAGAS decomposes an answer into
// before checking each one against context, just via punctuation rather
// than an LLM call.
func splitClaims(answer string) []string {
	var claims []string
	for _, s := range regexp.MustCompile(`[.!?]+`).Split(answer, -1) {
		s = strings.TrimSpace(s)
		if s != "" {
			claims = append(claims, s)
		}
	}
	if len(claims) == 0 {
		return []string{answer}
	}
	return claims
}

// Fake is a deterministic, free judge used by every test and the bundled
// demo — no API key, fully reproducible. It approximates each metric with
// word-overlap heuristics rather than real semantic understanding, which
// is a real limitation (see docs/ARCHITECTURE.md) but is sufficient to
// prove the harness's own logic (decomposition, aggregation, the CI gate)
// is correct, independent of judge quality.
type Fake struct {
	// SupportThreshold is the minimum fraction of a claim's distinct words
	// that must appear in the concatenated context for that claim to
	// count as supported.
	SupportThreshold float64
}

func NewFake() *Fake {
	return &Fake{SupportThreshold: 0.5}
}

func (f *Fake) ScoreFaithfulness(_ context.Context, _ string, answer string, contexts []string) (float64, error) {
	claims := splitClaims(answer)
	contextWords := words(strings.Join(contexts, " "))

	supported := 0
	for _, claim := range claims {
		claimWords := words(claim)
		if len(claimWords) == 0 {
			supported++ // an empty/punctuation-only "claim" trivially has nothing unsupported
			continue
		}
		hit := 0
		for w := range claimWords {
			if contextWords[w] {
				hit++
			}
		}
		if float64(hit)/float64(len(claimWords)) >= f.SupportThreshold {
			supported++
		}
	}
	return float64(supported) / float64(len(claims)), nil
}

func (f *Fake) ScoreAnswerRelevancy(_ context.Context, question, answer string) (float64, error) {
	return containment(words(question), words(answer)), nil
}
