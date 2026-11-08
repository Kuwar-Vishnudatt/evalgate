package gate

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadThresholds reads a .evalgate.yaml config file. This is what lets
// evalgate be committed once per repo as policy, rather than thresholds
// being hardcoded flags someone has to remember to pass correctly on
// every CI invocation.
func LoadThresholds(path string) (Thresholds, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Thresholds{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var th Thresholds
	if err := yaml.Unmarshal(data, &th); err != nil {
		return Thresholds{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return th, nil
}
