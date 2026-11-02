package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

type Client struct {
	apiKey  string
	baseURL string
	model   string
	dim     int
	hc      *http.Client
}

func New(apiKey, baseURL, model string, dim int) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		dim:     dim,
		hc:      &http.Client{Timeout: 30 * time.Second},
	}
}

type embedRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed 将单条文本向量化，并做 L2 归一化（使内积等价于余弦相似度）。
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	reqBody := embedRequest{Model: c.model, Input: texts}
	if c.dim > 0 {
		reqBody.Dimensions = c.dim
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		msg := resp.Status
		if out.Error != nil {
			msg = out.Error.Message
		}
		return nil, fmt.Errorf("embedding api error (%d): %s", resp.StatusCode, msg)
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embedding api returned %d vectors for %d inputs", len(out.Data), len(texts))
	}

	result := make([][]float32, len(texts))
	for i, d := range out.Data {
		v := make([]float32, len(d.Embedding))
		var norm float64
		for j, f := range d.Embedding {
			v[j] = float32(f)
			norm += f * f
		}
		if norm > 0 {
			inv := float32(1.0 / math.Sqrt(norm))
			for j := range v {
				v[j] *= inv
			}
		}
		result[i] = v
	}
	return result, nil
}
