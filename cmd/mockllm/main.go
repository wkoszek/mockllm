package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wkoszek/mockllm/mockserver"
)

const usage = `usage: mockllm <command> [args...]

Commands:
  server   start the mock server
  codex    run codex with OPENAI_BASE_URL pointed at the mock
  gemini   run gemini with OPENAI_BASE_URL pointed at the mock
  claude   run claude with OPENAI_BASE_URL pointed at the mock
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		runServer()
	case "codex", "gemini", "claude":
		runCLI(os.Args[1], os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func runServer() {
	cfg := mockserver.LoadConfigFromEnv()
	server, err := mockserver.New(cfg)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("mockllm listening on %s", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func runCLI(binary string, args []string) {
	path, err := exec.LookPath(binary)
	if err != nil {
		log.Fatalf("%s: not found in PATH", binary)
	}

	env := append(os.Environ(), mockEnvVars()...)
	if err := syscall.Exec(path, append([]string{binary}, args...), env); err != nil {
		log.Fatalf("exec %s: %v", binary, err)
	}
}

// mockEnvVars returns the environment variables that point a CLI at the mock server.
func mockEnvVars() []string {
	baseURL := "http://127.0.0.1" + listenPort()
	return []string{
		"OPENAI_BASE_URL=" + baseURL,
		"OPENAI_API_KEY=mock",
	}
}

// listenPort extracts the :port portion from OPENAI_MOCK_ADDR (default :8080).
func listenPort() string {
	addr := os.Getenv("OPENAI_MOCK_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	// addr is either ":8080" or "host:8080" — keep only the :port suffix.
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return addr[idx:]
	}
	return ":8080"
}
