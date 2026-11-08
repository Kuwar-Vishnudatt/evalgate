// Package metrics computes the two RAGAS metrics that are fundamentally
// set/rank comparisons against a ground truth, not semantic judgments:
// context precision (are the relevant contexts ranked near the top of
// what was retrieved?) and context recall (did retrieval find all the
// relevant contexts at all, regardless of rank?). Both require a golden
// case's RelevantContextIDs — the ground truth this package compares
// against — which is why they live separately from internal/judge's
// semantic metrics.
package metrics

// ContextPrecision computes RAGAS's rank-weighted precision@k: for every
// position in retrievedIDs that IS relevant, compute precision@that-rank
// (relevant-so-far / rank), then average those precision@k values over
// however many relevant positions there were — NOT over all retrieved
// positions. This rewards relevant contexts appearing EARLY: the same
// count of relevant hits scattered late in a long retrieved list scores
// lower than those same hits clustered at the top, which a naive
// unweighted "relevant retrieved / total retrieved" precision would not
// distinguish at all.
//
// If relevantIDs is empty, there is nothing to measure precision against;
// this returns 1.0 (vacuously perfect) rather than dividing by zero —
// callers should treat an empty ground-truth set as a golden-set
// authoring issue, not an evaluation failure.
func ContextPrecision(retrievedIDs []string, relevantIDs []string) float64 {
	if len(relevantIDs) == 0 {
		return 1.0
	}
	relevantSet := toSet(relevantIDs)

	var precisionSum float64
	relevantSoFar := 0
	relevantPositions := 0
	for i, id := range retrievedIDs {
		rank := i + 1
		if relevantSet[id] {
			relevantSoFar++
			relevantPositions++
			precisionSum += float64(relevantSoFar) / float64(rank)
		}
	}
	if relevantPositions == 0 {
		return 0.0 // relevant contexts exist in the golden set but NONE were retrieved at all
	}
	return precisionSum / float64(relevantPositions)
}

// ContextRecall is the fraction of relevantIDs that appear anywhere in
// retrievedIDs — order doesn't matter for recall, only whether retrieval
// found them at all. An empty relevantIDs set is treated as trivially
// fully recalled (1.0): there was nothing required to find, so retrieval
// cannot have failed to find it.
func ContextRecall(retrievedIDs []string, relevantIDs []string) float64 {
	if len(relevantIDs) == 0 {
		return 1.0
	}
	retrievedSet := toSet(retrievedIDs)
	found := 0
	for _, id := range relevantIDs {
		if retrievedSet[id] {
			found++
		}
	}
	return float64(found) / float64(len(relevantIDs))
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
