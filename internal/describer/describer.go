package describer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	describeMaxAttempts = 3
	describeRetryDelay  = 600 * time.Millisecond
	describeBodyLimit   = 800
)

type Config struct {
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
}

// Describe sends a face thumbnail to the LM Studio vision API and returns a text description.
func Describe(cfg Config, thumbnailPath string) (string, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("empty LM Studio endpoint")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return "", fmt.Errorf("empty LM Studio model")
	}

	imgData, err := os.ReadFile(thumbnailPath)
	if err != nil {
		return "", fmt.Errorf("read thumbnail: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(imgData)
	dataURL := "data:image/jpeg;base64," + b64

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 200
	}

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Describe this person briefly in 1-2 sentences: approximate age, gender, hair color, distinguishing features."},
					{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
				},
			},
		},
		"max_tokens":  maxTokens,
		"temperature": 0.1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(endpoint, "/") + "/chat/completions"
	client := &http.Client{Timeout: 120 * time.Second}
	var lastErr error
	for attempt := 1; attempt <= describeMaxAttempts; attempt++ {
		content, retryable, err := describeOnce(client, url, body)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !retryable || attempt == describeMaxAttempts {
			break
		}
		time.Sleep(time.Duration(attempt) * describeRetryDelay)
	}

	return "", fmt.Errorf("describe request failed after %d attempt(s): %w", describeMaxAttempts, lastErr)
}

func describeOnce(client *http.Client, url string, body []byte) (string, bool, error) {
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", true, fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return "", true, fmt.Errorf("temporary API error: %s, body: %s", resp.Status, compactBody(respBody))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("API request failed: %s, body: %s", resp.Status, compactBody(respBody))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", false, fmt.Errorf("decode response: %w, body: %s", err, compactBody(respBody))
	}

	if chatResp.Error != nil {
		return "", false, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", false, fmt.Errorf("empty response from model")
	}

	return chatResp.Choices[0].Message.Content, false, nil
}

func compactBody(body []byte) string {
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return "<empty>"
	}
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	if len(msg) > describeBodyLimit {
		return msg[:describeBodyLimit] + "...(truncated)"
	}
	return msg
}
