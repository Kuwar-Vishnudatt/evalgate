package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPJudge talks to any OpenAI-compatible /chat/completions endpoint,
// using the real RAGAS prompting strategy: decompose the answer into
// atomic claims and ask the judge whether each is supported by the given
// context (faithfulness), or rate 0-1 how directly the answer addresses
// the question (answer relevancy) — rather than one vague "rate this
// answer" prompt, which real RAGAS avoids for the same reason this does:
// a single holistic score conflates "well-written" with "faithful," and a
// fluent hallucination would score deceptively well.
type HTTPJudge struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
}

func NewHTTPJudge(baseURL, apiKey, model string) *HTTPJudge {
	return &HTTPJudge{BaseURL: baseURL, APIKey: apiKey, Model: model, Client: &http.Client{Timeout: 30 * time.Second}}
}

func (h *HTTPJudge) ScoreFaithfulness(ctx context.Context, question, answer string, contexts []string) (float64, error) {
	prompt := fmt.Sprintf(
		"Context:\n%s\n\nQuestion: %s\nAnswer: %s\n\n"+
			"Break the answer into its individual factual claims. For each claim, answer YES if it is "+
			"directly supported by the context, or NO if it is not stated or implied by the context. "+
			"Respond with ONLY a sequence of YES/NO, one per claim, space-separated, no other text.",
		strings.Join(contexts, "\n---\n"), question, answer,
	)
	resp, err := h.complete(ctx, prompt)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(strings.ToUpper(resp))
	if len(fields) == 0 {
		return 0, fmt.Errorf("faithfulness judge returned no verdicts")
	}
	yes := 0
	for _, f := range fields {
		if f == "YES" {
			yes++
		}
	}
	return float64(yes) / float64(len(fields)), nil
}

func (h *HTTPJudge) ScoreAnswerRelevancy(ctx context.Context, question, answer string) (float64, error) {
	prompt := fmt.Sprintf(
		"Question: %s\nAnswer: %s\n\n"+
			"On a scale from 0.0 (completely irrelevant) to 1.0 (directly and completely answers the question), "+
			"rate how relevant this answer is to the question. Respond with ONLY the number.",
		question, answer,
	)
	resp, err := h.complete(ctx, prompt)
	if err != nil {
		return 0, err
	}
	score, err := strconv.ParseFloat(strings.TrimSpace(resp), 64)
	if err != nil {
		return 0, fmt.Errorf("parse relevancy score %q: %w", resp, err)
	}
	return score, nil
}

func (h *HTTPJudge) complete(ctx context.Context, prompt string) (string, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"model":    h.Model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.APIKey)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("judge request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("judge request failed: %s: %s", resp.Status, body)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse judge response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("judge response had no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
