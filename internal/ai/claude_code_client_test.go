package ai

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct {
	stdout, stderr string
	err            error
	gotArgs        []string
	gotStdin       string
	gotEnv         []string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args []string, stdin string, extraEnv []string) (string, string, error) {
	f.gotArgs = args
	f.gotStdin = stdin
	f.gotEnv = extraEnv
	return f.stdout, f.stderr, f.err
}

func TestClaudeCode_GenerateSummary_ParsesResult(t *testing.T) {
	r := &fakeRunner{stdout: `{"type":"result","is_error":false,"result":"{\"summary\":\"resumo aqui\"}","usage":{"input_tokens":10,"output_tokens":5}}`}
	c := newClaudeCodeClientWithRunner("tok", "sonnet", r)
	got, in, out, err := c.GenerateSummary(context.Background(), "transcript", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "resumo aqui" || in != 10 || out != 5 {
		t.Fatalf("got %q in=%d out=%d", got, in, out)
	}
	if f := strings.Join(r.gotArgs, " "); !strings.Contains(f, "--output-format json") || !strings.Contains(f, "--model sonnet") {
		t.Fatalf("args: %v", r.gotArgs)
	}
	if !strings.Contains(r.gotStdin, "transcript") {
		t.Fatalf("prompt deve ir via stdin, foi: %q", r.gotStdin)
	}
	found := false
	for _, e := range r.gotEnv {
		if e == "CLAUDE_CODE_OAUTH_TOKEN=tok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("token ausente do env: %v", r.gotEnv)
	}
}

func TestClaudeCode_NoModel_OmitsModelFlag(t *testing.T) {
	r := &fakeRunner{stdout: `{"is_error":false,"result":"[\"p1\"]","usage":{}}`}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	if _, _, _, err := c.GenerateKeyPoints(context.Background(), "t", "", ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(r.gotArgs, " "), "--model") {
		t.Fatalf("--model não deveria aparecer: %v", r.gotArgs)
	}
}

func TestClaudeCode_CLIError_WrapsStderr(t *testing.T) {
	r := &fakeRunner{stderr: "boom", err: context.DeadlineExceeded}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	_, _, _, err := c.GenerateSummary(context.Background(), "t", "", "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("stderr deve entrar no erro, veio: %v", err)
	}
}

func TestClaudeCode_IsErrorTrue_ReturnsResultAsError(t *testing.T) {
	r := &fakeRunner{stdout: `{"is_error":true,"result":"OAuth token expired. Please run /login","usage":{}}`}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	_, _, _, err := c.GenerateSummary(context.Background(), "t", "", "")
	if err == nil || !IsAuthError(err) {
		t.Fatalf("erro de auth do CLI deve ser IsAuthError, veio: %v", err)
	}
}

func TestClaudeCode_MalformedCLIOutput_Errors(t *testing.T) {
	r := &fakeRunner{stdout: "not json"}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	if _, _, _, err := c.GenerateSummary(context.Background(), "t", "", ""); err == nil {
		t.Fatal("esperava erro de parse")
	}
}

func TestClaudeCode_GenerateTasks_ParsesArray(t *testing.T) {
	r := &fakeRunner{stdout: `{"is_error":false,"result":"[{\"description\":\"d\",\"assignee\":\"a\",\"priority\":\"high\"}]","usage":{"input_tokens":1,"output_tokens":2}}`}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	tasks, _, _, err := c.GenerateTasks(context.Background(), "t", "", "")
	if err != nil || len(tasks) != 1 || tasks[0].Priority != "high" {
		t.Fatalf("tasks=%v err=%v", tasks, err)
	}
}
