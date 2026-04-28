package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestApp_GetPort_AfterStartup(t *testing.T) {
	app := NewApp()
	app.OnStartup(context.Background())
	defer app.OnShutdown(context.Background())

	if app.GetPort() == 0 {
		t.Fatal("expected non-zero port after startup")
	}
}

func TestApp_HTTPServerResponds(t *testing.T) {
	app := NewApp()
	app.OnStartup(context.Background())
	defer app.OnShutdown(context.Background())

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", app.GetPort()))
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestApp_Shutdown_NoError(t *testing.T) {
	app := NewApp()
	app.OnStartup(context.Background())
	app.OnShutdown(context.Background())
}
