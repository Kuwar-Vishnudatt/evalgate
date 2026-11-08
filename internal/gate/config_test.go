package gate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadThresholds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".evalgate.yaml")
	content := `
min_faithfulness: 0.8
min_answer_relevancy: 0.3
min_context_precision: 0.7
min_context_recall: 0.7
max_regression: 0.05
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	th, err := LoadThresholds(path)
	if err != nil {
		t.Fatal(err)
	}
	if th.MinFaithfulness != 0.8 || th.MaxRegression != 0.05 {
		t.Fatalf("unexpected config: %+v", th)
	}
}

func TestLoadThresholdsMissingFile(t *testing.T) {
	if _, err := LoadThresholds("/nonexistent/path.yaml"); err == nil {
		t.Fatal("expected an error for a missing config file")
	}
}
