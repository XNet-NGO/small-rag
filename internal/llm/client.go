package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is an OpenAI-compatible HTTP client for LLM inference.
// It works with any OpenAI-compatible endpoint (OpenAI, Ollama, vLLM, llama.cpp server, LiteLLM).
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Message represents a chat message with a role and content.
type Message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// GetContent returns the content, falling back to reasoning_content for thinking models
func (m Message) GetContent() string {
	if m.Content != "" {
		return m.Content
	}
	return m.ReasoningContent
}

// ChatRequest represents a request to the chat completions endpoint.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

// ChatResponse represents the response from the chat completions endpoint.
type ChatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		Delta        Message `json:"delta"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// StreamCallback is called for each delta chunk during streaming.
// delta contains the content fragment, done indicates the stream is complete,
// and err is non-nil if an error occurred.
type StreamCallback func(delta string, done bool, err error)

// NewClient creates a new LLM client configured with the given base URL and optional API key.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ChatCompletion performs a non-streaming chat completion request.
func (c *Client) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("llm: failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatCompletionStream performs a streaming chat completion request.
// It reads SSE events from the upstream LLM and calls the callback for each delta chunk.
// Returns the final ChatResponse with usage stats if available.
func (c *Client) ChatCompletionStream(ctx context.Context, req ChatRequest, callback StreamCallback) (*ChatResponse, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var finalResponse ChatResponse
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Check for stream termination
		if line == "data: [DONE]" {
			callback("", true, nil)
			break
		}

		// Parse SSE data lines
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var chunk ChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			callback("", false, fmt.Errorf("llm: failed to parse stream chunk: %w", err))
			continue
		}

		// Capture usage stats if present
		if chunk.Usage.TotalTokens > 0 {
			finalResponse.Usage = chunk.Usage
		}

		// Call callback with delta content
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.GetContent()
			if delta != "" {
				callback(delta, false, nil)
			}

			// Capture finish reason
			if chunk.Choices[0].FinishReason != "" {
				finalResponse.Choices = chunk.Choices
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("llm: error reading stream: %w", err)
	}

	return &finalResponse, nil
}

// setHeaders sets common HTTP headers for requests.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}
