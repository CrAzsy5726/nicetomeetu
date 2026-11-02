package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	apiKey  string
	baseURL string
	model   string
	hc      *http.Client
}

func New(apiKey, baseURL, model string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		hc:      &http.Client{Timeout: 60 * time.Second},
	}
}

type chatRequest struct {
	Model          string      `json:"model"`
	Messages       []Message   `json:"messages"`
	Stream         bool        `json:"stream"`
	Temperature    float64     `json:"temperature,omitempty"`
	ResponseFormat *respFormat `json:"response_format,omitempty"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 单轮非流式补全。
func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	return c.chat(ctx, messages, false, nil, 0)
}

// ChatJSON 以 JSON 输出模式调用，返回原始 JSON 字符串（低温度，减少提取幻觉）。
func (c *Client) ChatJSON(ctx context.Context, messages []Message) (string, error) {
	return c.chat(ctx, messages, false, &respFormat{Type: "json_object"}, 0.1)
}

func (c *Client) chat(ctx context.Context, messages []Message, stream bool, rf *respFormat, temperature float64) (string, error) {
	reqBody := chatRequest{Model: c.model, Messages: messages, Stream: stream, Temperature: temperature, ResponseFormat: rf}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		msg := resp.Status
		if out.Error != nil {
			msg = out.Error.Message
		}
		return "", fmt.Errorf("llm api error (%d): %s", resp.StatusCode, msg)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm api returned no choices")
	}
	return out.Choices[0].Message.Content, nil
}

// StreamEvent 是流式输出的单个事件。
type StreamEvent struct {
	Delta string
	Done  bool
	Err   error
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// ChatStream 以 SSE 流式返回补全增量。返回的 channel 在完成后关闭。
func (c *Client) ChatStream(ctx context.Context, messages []Message) <-chan StreamEvent {
	out := make(chan StreamEvent)

	go func() {
		defer close(out)
		reqBody := chatRequest{Model: c.model, Messages: messages, Stream: true, Temperature: 0.8}
		b, err := json.Marshal(reqBody)
		if err != nil {
			out <- StreamEvent{Err: err}
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
		if err != nil {
			out <- StreamEvent{Err: err}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "text/event-stream")

		resp, err := c.hc.Do(req)
		if err != nil {
			out <- StreamEvent{Err: err}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			out <- StreamEvent{Err: fmt.Errorf("llm api error (%d)", resp.StatusCode)}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				out <- StreamEvent{Done: true}
				return
			}
			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) > 0 {
				if chunk.Choices[0].Delta.Content != "" {
					out <- StreamEvent{Delta: chunk.Choices[0].Delta.Content}
				}
				if chunk.Choices[0].FinishReason != nil {
					out <- StreamEvent{Done: true}
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- StreamEvent{Err: err}
			return
		}
		out <- StreamEvent{Done: true}
	}()

	return out
}
