package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type TaskSuggestion struct {
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	Priority    string `json:"priority"`
}

type AIClient interface {
	GenerateSummary(ctx context.Context, transcript string) (content string, inputTokens, outputTokens int, err error)
	GenerateKeyPoints(ctx context.Context, transcript string) (points []string, inputTokens, outputTokens int, err error)
	GenerateTasks(ctx context.Context, transcript string) (tasks []TaskSuggestion, inputTokens, outputTokens int, err error)
}

type AnthropicClient struct {
	client anthropic.Client
	model  string
}

func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicClient{client: c, model: model}
}

func (c *AnthropicClient) Model() string { return c.model }

func (c *AnthropicClient) GenerateSummary(ctx context.Context, transcript string) (string, int, int, error) {
	prompt := fmt.Sprintf(`Summarize the following meeting transcript in 2-3 paragraphs, in the same language as the transcript. Return ONLY a JSON object with the shape {"summary":"..."} and no extra text.

Transcript:
%s`, transcript)

	text, in, out, err := c.callJSON(ctx, prompt, 1024)
	if err != nil {
		return "", 0, 0, err
	}
	var result struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &result); err != nil {
		return "", 0, 0, fmt.Errorf("parse summary response: %w (raw: %s)", err, text)
	}
	return result.Summary, in, out, nil
}

func (c *AnthropicClient) GenerateKeyPoints(ctx context.Context, transcript string) ([]string, int, int, error) {
	prompt := fmt.Sprintf(`Extract the key points discussed in the following meeting transcript, in the same language as the transcript. Return ONLY a JSON array of strings: ["point 1","point 2",...] and no extra text.

Transcript:
%s`, transcript)

	text, in, out, err := c.callJSON(ctx, prompt, 1024)
	if err != nil {
		return nil, 0, 0, err
	}
	var points []string
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &points); err != nil {
		return nil, 0, 0, fmt.Errorf("parse key points response: %w (raw: %s)", err, text)
	}
	return points, in, out, nil
}

func (c *AnthropicClient) GenerateTasks(ctx context.Context, transcript string) ([]TaskSuggestion, int, int, error) {
	prompt := fmt.Sprintf(`Extract action items from the following meeting transcript, in the same language as the transcript. Return ONLY a JSON array with the shape [{"description":"...","assignee":"name or empty string","priority":"low|medium|high"},...] and no extra text.

Transcript:
%s`, transcript)

	text, in, out, err := c.callJSON(ctx, prompt, 2048)
	if err != nil {
		return nil, 0, 0, err
	}
	var tasks []TaskSuggestion
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &tasks); err != nil {
		return nil, 0, 0, fmt.Errorf("parse tasks response: %w (raw: %s)", err, text)
	}
	return tasks, in, out, nil
}

func (c *AnthropicClient) callJSON(ctx context.Context, prompt string, maxTokens int64) (string, int, int, error) {
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:       anthropic.Model(c.model),
		MaxTokens:   maxTokens,
		Temperature: anthropic.Float(0),
		System: []anthropic.TextBlockParam{
			{Text: "You are a JSON-only API. Output only valid JSON. No prose, no markdown fences, no extra text."},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("anthropic call: %w", err)
	}
	if len(msg.Content) == 0 {
		return "", 0, 0, fmt.Errorf("anthropic returned no content")
	}
	return msg.Content[0].Text, int(msg.Usage.InputTokens), int(msg.Usage.OutputTokens), nil
}

// stripJSONFence removes leading/trailing whitespace and ```json fences if present.
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}
