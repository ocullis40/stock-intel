package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/oliver/stock-intel/internal/types"
)

const anthropicAPI = "https://api.anthropic.com/v1/messages"

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	Tools     []apiTool    `json:"tools"`
	Messages  []apiMessage `json:"messages"`
}

type apiTool struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type apiResponse struct {
	Content []apiContentBlock `json:"content"`
	Usage   apiUsage          `json:"usage"`
	Error   *apiError         `json:"error,omitempty"`
}

type apiContentBlock struct {
	Type    string             `json:"type"`
	Text    string             `json:"text,omitempty"`
	Content []apiSearchResult  `json:"content,omitempty"` // for web_search_tool_result blocks
}

type apiSearchResult struct {
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	URL       string `json:"url,omitempty"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// SearchAndExtract calls Claude with web_search enabled and returns
// the concatenated text blocks, citation sources, and token usage.
func SearchAndExtract(prompt string, model string) (string, []types.Source, types.Usage, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", nil, types.Usage{}, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	reqBody := apiRequest{
		Model:     model,
		MaxTokens: 1600,
		Tools: []apiTool{
			{Type: "web_search_20250305", Name: "web_search"},
		},
		Messages: []apiMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, types.Usage{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", anthropicAPI, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", nil, types.Usage{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	var resp *http.Response
	var respBytes []byte
	maxRetries := 3
	for attempt := range maxRetries {
		reqCopy, _ := http.NewRequest("POST", anthropicAPI, bytes.NewReader(bodyBytes))
		reqCopy.Header.Set("Content-Type", "application/json")
		reqCopy.Header.Set("x-api-key", apiKey)
		reqCopy.Header.Set("anthropic-version", "2023-06-01")

		resp, err = http.DefaultClient.Do(reqCopy)
		if err != nil {
			return "", nil, types.Usage{}, fmt.Errorf("http request: %w", err)
		}

		respBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", nil, types.Usage{}, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode == 429 && attempt < maxRetries-1 {
			wait := 30 * time.Second
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if secs, err := strconv.Atoi(retryAfter); err == nil {
					wait = time.Duration(secs) * time.Second
				}
			}
			fmt.Printf("  Rate limited, waiting %s before retry...\n", wait)
			time.Sleep(wait)
			continue
		}
		break
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil, types.Usage{}, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return "", nil, types.Usage{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return "", nil, types.Usage{}, fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	u := types.Usage{
		InputTokens:  apiResp.Usage.InputTokens,
		OutputTokens: apiResp.Usage.OutputTokens,
	}

	var texts []string
	seen := map[string]bool{}
	var sources []types.Source
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
		if block.Type == "web_search_tool_result" {
			for _, r := range block.Content {
				if r.Type == "web_search_result" && r.URL != "" && !seen[r.URL] {
					seen[r.URL] = true
					sources = append(sources, types.Source{Title: r.Title, URL: r.URL})
				}
			}
		}
	}

	return strings.Join(texts, ""), sources, u, nil
}

// ParseJSON extracts and parses JSON from a Claude response, handling
// markdown fences and any surrounding text.
func ParseJSON[T any](raw string) (*T, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, "```json", "")
	cleaned = strings.ReplaceAll(cleaned, "```", "")
	cleaned = strings.TrimSpace(cleaned)

	// If direct parse fails, try to extract the JSON object from surrounding text.
	var result T
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		start := strings.Index(cleaned, "{")
		end := strings.LastIndex(cleaned, "}")
		if start >= 0 && end > start {
			extracted := cleaned[start : end+1]
			if err2 := json.Unmarshal([]byte(extracted), &result); err2 != nil {
				return nil, fmt.Errorf("parse JSON: %w (raw: %.200s)", err2, cleaned)
			}
			return &result, nil
		}
		return nil, fmt.Errorf("parse JSON: %w (raw: %.200s)", err, cleaned)
	}
	return &result, nil
}
