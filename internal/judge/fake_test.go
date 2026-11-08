package judge

import (
	"context"
	"testing"
)

func TestFaithfulnessHighWhenAnswerClaimsAreInContext(t *testing.T) {
	f := NewFake()
	answer := "The warranty lasts two years. Returns are accepted within thirty days."
	contexts := []string{"Our product warranty lasts two years from purchase. Returns are accepted within thirty days of delivery."}

	score, err := f.ScoreFaithfulness(context.Background(), "q", answer, contexts)
	if err != nil {
		t.Fatal(err)
	}
	if score < 0.9 {
		t.Fatalf("expected high faithfulness when every claim is supported by context, got %v", score)
	}
}

func TestFaithfulnessLowWhenAnswerHallucinates(t *testing.T) {
	f := NewFake()
	answer := "The warranty lasts ten years and covers accidental damage from meteor strikes."
	contexts := []string{"Our product warranty lasts two years from purchase. Returns are accepted within thirty days of delivery."}

	score, err := f.ScoreFaithfulness(context.Background(), "q", answer, contexts)
	if err != nil {
		t.Fatal(err)
	}
	if score > 0.5 {
		t.Fatalf("expected low faithfulness for a largely hallucinated answer, got %v", score)
	}
}

func TestFaithfulnessIsPerClaimNotWholeAnswer(t *testing.T) {
	// One supported claim and one hallucinated claim should land roughly
	// in the middle, proving claims are scored independently rather than
	// the whole answer being judged as one blob.
	f := NewFake()
	answer := "The warranty lasts two years. It also includes free unicorn insurance."
	contexts := []string{"Our product warranty lasts two years from purchase."}

	score, err := f.ScoreFaithfulness(context.Background(), "q", answer, contexts)
	if err != nil {
		t.Fatal(err)
	}
	if score <= 0 || score >= 1 {
		t.Fatalf("expected a partial score for a half-supported answer, got %v", score)
	}
}

func TestAnswerRelevancyHighForOnTopicAnswer(t *testing.T) {
	f := NewFake()
	score, err := f.ScoreAnswerRelevancy(context.Background(), "how long is the warranty", "the warranty is two years long")
	if err != nil {
		t.Fatal(err)
	}
	if score < 0.3 {
		t.Fatalf("expected meaningful overlap for an on-topic answer, got %v", score)
	}
}

func TestAnswerRelevancyLowForOffTopicAnswer(t *testing.T) {
	f := NewFake()
	score, err := f.ScoreAnswerRelevancy(context.Background(), "how long is the warranty", "bananas are a good source of potassium")
	if err != nil {
		t.Fatal(err)
	}
	if score > 0.1 {
		t.Fatalf("expected near-zero overlap for a completely off-topic answer, got %v", score)
	}
}

func TestAnswerRelevancyNotPenalizedForBeingMoreDetailedThanQuestion(t *testing.T) {
	// A real-world regression found via the example-target end-to-end
	// test: symmetric Jaccard similarity scores a short question against
	// a long, fully-relevant, fully-supported answer as LOW relevancy,
	// purely because the answer's union of words is much bigger than the
	// question's — penalizing verbosity, not irrelevance. Containment
	// (fraction of the QUESTION's words found in the answer) must not
	// have this failure mode.
	f := NewFake()
	question := "How long is the warranty?"
	answer := "Our product warranty lasts two years from the date of purchase, covering manufacturing defects but not accidental damage."
	score, err := f.ScoreAnswerRelevancy(context.Background(), question, answer)
	if err != nil {
		t.Fatal(err)
	}
	if score < 0.3 {
		t.Fatalf("expected a detailed, genuinely relevant answer to score reasonably even though it's far longer than the question, got %v", score)
	}
}

func TestSplitClaimsHandlesEmptyAnswer(t *testing.T) {
	claims := splitClaims("")
	if len(claims) != 1 {
		t.Fatalf("expected a fallback single claim for an empty answer, got %v", claims)
	}
}

func TestSplitClaimsHandlesSingleSentence(t *testing.T) {
	claims := splitClaims("This is one sentence")
	if len(claims) != 1 {
		t.Fatalf("expected exactly one claim, got %v", claims)
	}
}
