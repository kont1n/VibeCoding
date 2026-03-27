package describer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Config struct {
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
}

// Describe sends a face thumbnail to the LM Studio vision API and returns a text description.
func Describe(cfg Config, thumbnailPath string) (string, error) {
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
		"model": cfg.Model,
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

	url := cfg.Endpoint + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
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
		return "", fmt.Errorf("decode response: %w, body: %s", err, string(respBody))
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from model")
	}

	return chatResp.Choices[0].Message.Content, nil
}
