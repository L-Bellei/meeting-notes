package ai

import (
	"context"
	"testing"
)

func TestLaunchSetupToken_UsesVisibleConsole(t *testing.T) {
	orig := launchConsole
	defer func() { launchConsole = orig }()
	var gotBin string
	launchConsole = func(bin string) error { gotBin = bin; return nil }
	if err := LaunchSetupToken(); err != nil {
		t.Fatal(err)
	}
	if gotBin == "" {
		t.Fatal("binário não resolvido")
	}
}

func TestTestConnection_UsesTokenAndModel(t *testing.T) {
	r := &fakeRunner{stdout: `{"is_error":false,"result":"{\"ok\":true}","usage":{}}`}
	if err := testConnectionWithRunner(context.Background(), "tok", "sonnet", r); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range r.gotEnv {
		if e == "CLAUDE_CODE_OAUTH_TOKEN=tok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("token ausente: %v", r.gotEnv)
	}
}

func TestTestConnection_AuthError(t *testing.T) {
	r := &fakeRunner{stdout: `{"is_error":true,"result":"OAuth token expired. Please run /login","usage":{}}`}
	err := testConnectionWithRunner(context.Background(), "tok", "", r)
	if err == nil || !IsAuthError(err) {
		t.Fatalf("esperava auth error, veio %v", err)
	}
}
