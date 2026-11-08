// Package example ships a minimal product-FAQ answering target — real
// enough to exercise all four metrics end to end, deliberately not a
// serious retrieval system (that's not what this project is about; see
// docs/ARCHITECTURE.md). A Good variant retrieves the top-3 most relevant
// documents and answers only using their sentences; a Degraded variant
// retrieves just the top-1 (hurting context recall/precision) and
// appends a fabricated, unsupported sentence to every answer (hurting
// faithfulness) — simulating a realistic two-dimensional regression: a
// retrieval config change PLUS a prompt that over-generates, together,
// which is exactly the kind of regression a single eyeballed answer is
// unlikely to catch but an aggregate metric drop will.
package example

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/Kuwar-Vishnudatt/evalgate/internal/target"
)

type Doc struct {
	ID   string
	Text string
}

// FAQDocs is the bundled example knowledge base: a small, fictional
// product's FAQ, just large enough that retrieval and faithfulness are
// both genuinely exercised rather than trivial.
func FAQDocs() []Doc {
	return []Doc{
		{ID: "warranty", Text: "Our product warranty lasts two years from the date of purchase. It covers manufacturing defects but not accidental damage."},
		{ID: "returns", Text: "Returns are accepted within thirty days of delivery for a full refund, provided the item is unused."},
		{ID: "shipping", Text: "Standard shipping takes five to seven business days within the continental United States."},
		{ID: "support", Text: "Customer support is available Monday through Friday, nine AM to six PM Eastern time."},
		{ID: "pricing", Text: "Pricing includes a one-time setup fee of fifty dollars plus a monthly subscription of twenty dollars."},
	}
}

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
		m[w] = true
	}
	return m
}

func overlapScore(a, b map[string]bool) int {
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	return n
}

func retrieve(docs []Doc, question string, topK int) []Doc {
	qWords := wordSet(question)
	type scored struct {
		doc   Doc
		score int
	}
	var ranked []scored
	for _, d := range docs {
		ranked = append(ranked, scored{d, overlapScore(qWords, wordSet(d.Text))})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	if topK > len(ranked) {
		topK = len(ranked)
	}
	out := make([]Doc, topK)
	for i := 0; i < topK; i++ {
		out[i] = ranked[i].doc
	}
	return out
}

// GoodTarget retrieves the top-3 documents and answers using only their
// text, concatenated — every claim it makes is, by construction, drawn
// directly from retrieved context.
type GoodTarget struct {
	Docs []Doc
}

func (g GoodTarget) Generate(_ context.Context, question string) (target.Response, error) {
	top := retrieve(g.Docs, question, 3)
	var answerParts []string
	contexts := make([]target.Context, len(top))
	for i, d := range top {
		answerParts = append(answerParts, firstSentence(d.Text))
		contexts[i] = target.Context{ID: d.ID, Text: d.Text}
	}
	return target.Response{Answer: strings.Join(answerParts, " "), Contexts: contexts}, nil
}

// DegradedTarget simulates a realistic regression: retrieves only the
// single best-matching document (worse recall/precision when the real
// answer needs more than one) and always appends a fabricated claim no
// retrieved document supports (worse faithfulness).
type DegradedTarget struct {
	Docs []Doc
}

func (d DegradedTarget) Generate(_ context.Context, question string) (target.Response, error) {
	top := retrieve(d.Docs, question, 1)
	var answerParts []string
	contexts := make([]target.Context, len(top))
	for i, doc := range top {
		answerParts = append(answerParts, firstSentence(doc.Text))
		contexts[i] = target.Context{ID: doc.ID, Text: doc.Text}
	}
	answerParts = append(answerParts, "This offer is also available retroactively for all purchases made before the company existed.")
	return target.Response{Answer: strings.Join(answerParts, " "), Contexts: contexts}, nil
}

func firstSentence(text string) string {
	if idx := strings.IndexAny(text, ".!?"); idx >= 0 {
		return strings.TrimSpace(text[:idx+1])
	}
	return text
}
