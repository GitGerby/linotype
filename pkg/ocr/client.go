package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Client talks to an OpenAI-compatible vision API.
type Client struct {
	BaseURL     string
	APIKey      string
	Model       string
	HTTPClient  *http.Client
	Debug       bool
	PNGExport   string
	exportCount atomic.Uint64
}

// NewClient creates a new OCR client.
func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Model:      model,
		HTTPClient: &http.Client{},
	}
}

type ocrRequest struct {
	Model     string       `json:"model"`
	Messages  []ocrMessage `json:"messages"`
	MaxTokens int          `json:"max_tokens"`
}

type ocrMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type imageContent struct {
	Type     string `json:"type"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

type ocrResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Recognize sends a PNG image to the LLM and returns the OCR'd text.
// The systemPrompt provides instructions to the model.
func (c *Client) Recognize(ctx context.Context, pngData []byte, systemPrompt string) (string, error) {
	if c.Debug && c.PNGExport != "" {
		if err := os.MkdirAll(c.PNGExport, 0755); err != nil {
			return "", fmt.Errorf("failed to create PNG export directory: %w", err)
		}
		idx := c.exportCount.Add(1)
		filename := fmt.Sprintf("sub_%05d.png", idx)
		filePath := filepath.Join(c.PNGExport, filename)
		if err := os.WriteFile(filePath, pngData, 0644); err != nil {
			return "", fmt.Errorf("failed to export debug PNG: %w", err)
		}
	}

	b64Image := base64.StdEncoding.EncodeToString(pngData)
	imageURL := fmt.Sprintf("data:image/png;base64,%s", b64Image)

	msgContent := []interface{}{
		imageContent{
			Type: "image_url",
			ImageURL: struct {
				URL string `json:"url"`
			}{URL: imageURL},
		},
	}

	reqBody := ocrRequest{
		Model: c.Model,
		Messages: []ocrMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: msgContent},
		},
		MaxTokens: 8192,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.BaseURL
	if !strings.HasSuffix(url, "/chat/completions") {
		url = strings.TrimRight(url, "/") + "/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		snippet := string(bodyBytes)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, snippet)
	}

	var ocrResp ocrResponse
	if err := json.Unmarshal(bodyBytes, &ocrResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(ocrResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices in response")
	}

	return strings.TrimSpace(ocrResp.Choices[0].Message.Content), nil
}

// ExportCount returns the number of PNG files exported so far.
func (c *Client) ExportCount() uint64 {
	return c.exportCount.Load()
}

// ResetExportCount resets the export counter.
func (c *Client) ResetExportCount() {
	c.exportCount.Store(0)
}
