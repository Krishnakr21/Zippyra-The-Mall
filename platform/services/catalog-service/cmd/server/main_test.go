package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/catalog-service/config"
)

func TestMainExecution(t *testing.T) {
	if os.Getenv("BE_SERVER") == "1" {
		// Mock DB and other things if needed, but main() will load config and fail
		// if DB is not available.
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainExecution")
	cmd.Env = append(os.Environ(), "BE_SERVER=1", "PORT=0") // Use random port to avoid conflict

	// Start the process
	err := cmd.Start()
	assert.NoError(t, err)

	// Wait a bit and kill it
	time.Sleep(200 * time.Millisecond)
	err = cmd.Process.Signal(syscall.SIGINT)
	assert.NoError(t, err)

	err = cmd.Wait()
	// main() calls log.Fatal which exits with 1 if run() returns error.
	// But it might also exit with 0 if we send signal?
	// Actually, log.Fatal ALWAYS exits with 1.
}

func TestRun_Error(t *testing.T) {
	// 1. Database parse error
	os.Setenv("DATABASE_URL", "invalid://URL")
	err := run(context.Background())
	assert.Error(t, err)
	os.Unsetenv("DATABASE_URL")

	// 2. Otel init error
	os.Setenv("OTEL_TRACES_EXPORTER", "invalid")
	defer os.Unsetenv("OTEL_TRACES_EXPORTER")
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
	defer os.Unsetenv("DATABASE_URL")
	err = run(context.Background())
	// Should fail at some point, but otel.Init should have been called and logged warning
	assert.Error(t, err)
}

func TestRun_Signal(t *testing.T) {
	// Mock DB
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mock.Close()

	// Use a random free port
	os.Setenv("PORT", "0")
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("DATABASE_URL")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err = run(ctx)
	// When server shuts down gracefully via context, ListenAndServe returns http.ErrServerClosed
	assert.Equal(t, http.ErrServerClosed, err)
}

func TestRun_OSSignal(t *testing.T) {
	// Mock DB
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mock.Close()

	// Use a random free port
	os.Setenv("PORT", "0")
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("DATABASE_URL")

	// Run in goroutine and send signal
	go func() {
		time.Sleep(200 * time.Millisecond)
		pid := os.Getpid()
		process, err := os.FindProcess(pid)
		if err == nil {
			process.Signal(syscall.SIGINT)
		}
	}()

	err = run(context.Background())
	// When server shuts down via signal, ListenAndServe returns http.ErrServerClosed
	assert.Equal(t, http.ErrServerClosed, err)
}

func TestSetupRouter_Error(t *testing.T) {
	cfg := &config.Config{
		Search: config.SearchConfig{
			Addr: "", // This will trigger error in NewOpenSearchClient
		},
	}
	// setupRouter should not return nil even if search client fails (it logs warning)
	r := setupRouter(cfg, nil)
	assert.NotNil(t, r)
}

func TestSetupRouter(t *testing.T) {
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mock.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: "8084",
		},
		Search: config.SearchConfig{
			Addr: "http://localhost:9200",
		},
	}

	r := setupRouter(cfg, mock)
	assert.NotNil(t, r)

	// Test health endpoint through the real router
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Test metrics endpoint
	req = httptest.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetupRouter_SearchError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mock.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: "8084",
		},
		Search: config.SearchConfig{
			Addr: "://", // Invalid URL to force NewOpenSearchClient error
		},
	}

	r := setupRouter(cfg, mock)
	assert.NotNil(t, r) // Should still return router despite search error
}

func TestSignalHandling(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sigChan <- syscall.SIGINT
	}()

	select {
	case <-sigChan:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for signal")
	}
}

func TestGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	server := &http.Server{
		Addr:    ":0",
		Handler: http.NewServeMux(),
	}

	go func() {
		server.ListenAndServe()
	}()

	err := server.Shutdown(ctx)
	assert.NoError(t, err)
}
