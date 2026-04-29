package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

type OpenAIClient struct {
	client openai.Client
	model  string
}

func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	c := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIClient{client: c, model: model}
}

func (c *OpenAIClient) GenerateSummary(ctx context.Context, transcript, notes, customPrompt string) (string, int, int, error) {
	const jsonFmt = `Return ONLY a JSON object with the shape {"summary":"..."} and no extra text.`
	const def = `Summarize the following meeting content in 2-3 paragraphs, in the same language as the content.`
	prompt := fmt.Sprintf("%s %s\n\nContent:\n%s", buildInstruction(def, customPrompt), jsonFmt, buildContext(transcript, notes))

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

func (c *OpenAIClient) GenerateKeyPoints(ctx context.Context, transcript, notes, customPrompt string) ([]string, int, int, error) {
	const jsonFmt = `Return ONLY a JSON array of strings: ["point 1","point 2",...] and no extra text.`
	const def = `Extract the key points discussed in the following meeting content, in the same language as the content.`
	prompt := fmt.Sprintf("%s %s\n\nContent:\n%s", buildInstruction(def, customPrompt), jsonFmt, buildContext(transcript, notes))

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

func (c *OpenAIClient) GenerateTasks(ctx context.Context, transcript, notes, customPrompt string) ([]TaskSuggestion, int, int, error) {
	const jsonFmt = `Return ONLY a JSON array with the shape [{"description":"...","assignee":"name or empty string","priority":"low|medium|high"},...] and no extra text.`
	const def = `Extract action items from the following meeting content, in the same language as the content.`
	prompt := fmt.Sprintf("%s %s\n\nContent:\n%s", buildInstruction(def, customPrompt), jsonFmt, buildContext(transcript, notes))

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

func (c *OpenAIClient) callJSON(ctx context.Context, prompt string, maxTokens int) (string, int, int, error) {
	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:       openai.ChatModel(c.model),
		MaxTokens:   param.NewOpt(int64(maxTokens)),
		Temperature: param.NewOpt(float64(0)),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a JSON-only API. Output only valid JSON. No prose, no markdown fences, no extra text."),
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("openai call: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("openai returned no choices")
	}
	in := int(resp.Usage.PromptTokens)
	out := int(resp.Usage.CompletionTokens)
	return resp.Choices[0].Message.Content, in, out, nil
}
