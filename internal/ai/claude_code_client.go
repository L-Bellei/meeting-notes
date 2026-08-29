package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const generateTimeout = 5 * time.Minute

type commandRunner interface {
	Run(ctx context.Context, name string, args []string, stdin string, extraEnv []string) (stdout, stderr string, err error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args []string, stdin string, extraEnv []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}

func findClaudeBinary() (string, error) {
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		// Instalador nativo do Windows coloca o binário fora do PATH de processos GUI.
		p := filepath.Join(home, ".local", "bin", "claude.exe")
		if _, statErr := os.Stat(p); statErr == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w (binário claude não encontrado — instale o Claude Code)", ErrNotConfigured)
}

type ClaudeCodeClient struct {
	token  string
	model  string
	runner commandRunner
}

func NewClaudeCodeClient(token, model string) *ClaudeCodeClient {
	return newClaudeCodeClientWithRunner(token, model, execRunner{})
}

func newClaudeCodeClientWithRunner(token, model string, r commandRunner) *ClaudeCodeClient {
	return &ClaudeCodeClient{token: token, model: model, runner: r}
}

type cliResult struct {
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *ClaudeCodeClient) callJSON(ctx context.Context, prompt string) (string, int, int, error) {
	bin, err := findClaudeBinary()
	if err != nil {
		return "", 0, 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()
	args := []string{"-p", "--output-format", "json",
		"--append-system-prompt", "You are a JSON-only API. Output only valid JSON. No prose, no markdown fences, no extra text."}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}
	stdout, stderr, err := c.runner.Run(ctx, bin, args, prompt, []string{"CLAUDE_CODE_OAUTH_TOKEN=" + c.token})
	if err != nil {
		return "", 0, 0, fmt.Errorf("claude cli: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}
	var res cliResult
	if jsonErr := json.Unmarshal([]byte(stdout), &res); jsonErr != nil {
		return "", 0, 0, fmt.Errorf("parse claude cli output: %w (raw: %.200s)", jsonErr, stdout)
	}
	if res.IsError {
		return "", 0, 0, fmt.Errorf("claude cli error: %s", res.Result)
	}
	return res.Result, res.Usage.InputTokens, res.Usage.OutputTokens, nil
}

func (c *ClaudeCodeClient) GenerateSummary(ctx context.Context, transcript, notes, customPrompt string) (string, int, int, error) {
	const jsonFmt = `Return ONLY a JSON object with the shape {"summary":"..."} and no extra text.`
	const def = `Summarize the following meeting content in 2-3 paragraphs, in the same language as the content.`
	prompt := fmt.Sprintf("%s %s\n\nContent:\n%s", buildInstruction(def, customPrompt), jsonFmt, buildContext(transcript, notes))

	text, in, out, err := c.callJSON(ctx, prompt)
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

func (c *ClaudeCodeClient) GenerateKeyPoints(ctx context.Context, transcript, notes, customPrompt string) ([]string, int, int, error) {
	const jsonFmt = `Return ONLY a JSON array of strings: ["point 1","point 2",...] and no extra text.`
	const def = `Extract the key points discussed in the following meeting content, in the same language as the content.`
	prompt := fmt.Sprintf("%s %s\n\nContent:\n%s", buildInstruction(def, customPrompt), jsonFmt, buildContext(transcript, notes))

	text, in, out, err := c.callJSON(ctx, prompt)
	if err != nil {
		return nil, 0, 0, err
	}
	var points []string
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &points); err != nil {
		return nil, 0, 0, fmt.Errorf("parse key points response: %w (raw: %s)", err, text)
	}
	return points, in, out, nil
}

func (c *ClaudeCodeClient) GenerateTasks(ctx context.Context, transcript, notes, customPrompt string) ([]TaskSuggestion, int, int, error) {
	const jsonFmt = `Return ONLY a JSON array with the shape [{"description":"...","assignee":"name or empty string","priority":"low|medium|high"},...] and no extra text.`
	const def = `Extract action items from the following meeting content, in the same language as the content.`
	prompt := fmt.Sprintf("%s %s\n\nContent:\n%s", buildInstruction(def, customPrompt), jsonFmt, buildContext(transcript, notes))

	text, in, out, err := c.callJSON(ctx, prompt)
	if err != nil {
		return nil, 0, 0, err
	}
	var tasks []TaskSuggestion
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &tasks); err != nil {
		return nil, 0, 0, fmt.Errorf("parse tasks response: %w (raw: %s)", err, text)
	}
	return tasks, in, out, nil
}
