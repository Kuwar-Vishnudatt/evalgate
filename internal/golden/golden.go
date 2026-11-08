// Package golden defines the versioned golden test set a RAG application
// is evaluated against: a question, the ideal answer (used only as
// reference text in prompts, never string-matched against), and the set
// of context IDs that genuinely answer the question — the ground truth
// context-precision/recall is computed against.
package golden

import (
	"encoding/json"
	"fmt"
	"os"
)

type Case struct {
	ID                 string   `json:"id"`
	Question           string   `json:"question"`
	GroundTruthAnswer  string   `json:"ground_truth_answer"`
	RelevantContextIDs []string `json:"relevant_context_ids"`
}

// Load reads a golden set from a JSON file: an array of Case.
func Load(path string) ([]Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read golden set %s: %w", path, err)
	}
	var cases []Case
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("parse golden set %s: %w", path, err)
	}
	seen := map[string]bool{}
	for _, c := range cases {
		if c.ID == "" {
			return nil, fmt.Errorf("golden set %s: a case is missing an id", path)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("golden set %s: duplicate case id %q", path, c.ID)
		}
		seen[c.ID] = true
		if c.Question == "" {
			return nil, fmt.Errorf("golden set %s: case %q is missing a question", path, c.ID)
		}
	}
	return cases, nil
}
