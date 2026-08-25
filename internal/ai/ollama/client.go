package ollama

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/ai"
	"github.com/Homiakus/Home_Sentinel/internal/ai/prompts"
)

const maxBody = 16 << 20

type Client struct {
	base  *url.URL
	model string
	http  *http.Client
}

type Options struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

func New(opts Options) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(opts.BaseURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("Ollama base URL invalid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("Ollama base URL must use http or https")
	}
	if strings.TrimSpace(opts.Model) == "" {
		return nil, errors.New("Ollama model required")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{base: u, model: opts.Model, http: hc}, nil
}

func (c *Client) endpoint(path string) string {
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + path
	return u.String()
}
func (c *Client) Health(ctx context.Context) ai.Health {
	var out struct {
		Version string `json:"version"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/version", nil, &out); err != nil {
		return ai.Health{Error: err.Error()}
	}
	return ai.Health{Reachable: true, Version: out.Version}
}
func (c *Client) Models(ctx context.Context) ([]ai.Model, error) {
	var out struct {
		Models []struct {
			Name       string    `json:"name"`
			Size       int64     `json:"size"`
			Digest     string    `json:"digest"`
			ModifiedAt time.Time `json:"modified_at"`
			Details    struct {
				Family        string `json:"family"`
				ParameterSize string `json:"parameter_size"`
				Quantization  string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/tags", nil, &out); err != nil {
		return nil, err
	}
	res := make([]ai.Model, 0, len(out.Models))
	for _, m := range out.Models {
		res = append(res, ai.Model{Name: m.Name, SizeBytes: m.Size, Digest: m.Digest, ModifiedAt: m.ModifiedAt, Family: m.Details.Family, ParameterSize: m.Details.ParameterSize, QuantizationLevel: m.Details.Quantization})
	}
	return res, nil
}

func (c *Client) Analyze(ctx context.Context, req ai.AnalysisRequest) (ai.AnalysisResult, error) {
	if len(req.Frames) == 0 {
		return ai.AnalysisResult{}, errors.New("AI analysis requires at least one frame")
	}
	images := make([]string, 0, len(req.Frames))
	for _, f := range req.Frames {
		if len(f.JPEG) == 0 {
			continue
		}
		images = append(images, base64.StdEncoding.EncodeToString(f.JPEG))
	}
	if len(images) == 0 {
		return ai.AnalysisResult{}, errors.New("AI frames are empty")
	}
	prompt := prompts.EventAnalysisPrompt
	if strings.TrimSpace(req.Prompt) != "" {
		prompt += "\nAdditional task context: " + strings.TrimSpace(req.Prompt)
	}
	body := map[string]any{"model": c.model, "messages": []map[string]any{{"role": "user", "content": prompt, "images": images}}, "format": json.RawMessage(prompts.EventAnalysisSchema), "stream": false, "options": map[string]any{"temperature": 0}}
	var out struct {
		Model   string `json:"model"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		TotalDuration   int64 `json:"total_duration"`
		LoadDuration    int64 `json:"load_duration"`
		PromptEvalCount int   `json:"prompt_eval_count"`
		EvalCount       int   `json:"eval_count"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/chat", body, &out); err != nil {
		return ai.AnalysisResult{}, err
	}
	var result ai.AnalysisResult
	if err := json.Unmarshal([]byte(out.Message.Content), &result); err != nil {
		return ai.AnalysisResult{}, fmt.Errorf("decode structured Ollama result: %w", err)
	}
	result.Model = out.Model
	result.RawDescription = out.Message.Content
	result.TotalDuration = time.Duration(out.TotalDuration)
	result.LoadDuration = time.Duration(out.LoadDuration)
	result.Inference = result.TotalDuration - result.LoadDuration
	result.PromptTokens = out.PromptEvalCount
	result.GeneratedTokens = out.EvalCount
	if result.Inference < 0 {
		result.Inference = 0
	}
	if err := result.Validate(); err != nil {
		return ai.AnalysisResult{}, err
	}
	return result, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if c == nil || c.base == nil {
		return errors.New("Ollama client unavailable")
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path), r)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return err
	}
	if len(data) > maxBody {
		return errors.New("Ollama response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Ollama %s returned HTTP %d", path, resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode Ollama response: %w", err)
		}
	}
	return nil
}
