// Package main provides the lift CLI entrypoint.
// This is the main binary for the Lift CLI tool.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pay-theory/lift/pkg/cli"
)

// Version is set at build time via ldflags
var Version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	// Create context with cancellation on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	defer signal.Stop(sigCh)

	// Create CLI with version
	app := cli.NewCLI(Version)

	// Execute with command line arguments (skip program name)
	if err := app.Execute(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}
