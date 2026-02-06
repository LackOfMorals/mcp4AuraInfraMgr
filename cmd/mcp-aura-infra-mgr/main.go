package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/LackOfMorals/mcp4AuraAPI/internal/cli"
	"github.com/LackOfMorals/mcp4AuraAPI/internal/config"
	"github.com/LackOfMorals/mcp4AuraAPI/internal/logger"
	"github.com/LackOfMorals/mcp4AuraAPI/internal/server"
)

// go build -C cmd/neo4j-mcp -o ../../bin/ -ldflags "-X 'main.Version=9999'"
var Version = "development"

func main() {
	// Handle CLI arguments (version, help, etc.)
	cli.HandleArgs(Version)

	// Parse CLI flags for configuration
	cliArgs := cli.ParseConfigFlags()

	// Load and validate configuration (env vars + CLI overrides)
	cfg, err := config.LoadConfig(&config.CLIOverrides{
		URI:       cliArgs.URI,
		ReadOnly:  cliArgs.ReadOnly,
		LogLevel:  cliArgs.LogLevel,
		LogFormat: cliArgs.LogFormat,
	})
	if err != nil {
		// Can't use logger here yet, so just print to stderr
		fmt.Fprintln(os.Stderr, "Failed to load configuration: "+err.Error())
		os.Exit(1)
	}

	// Initialize global logger
	logger.Init(cfg.LogLevel, cfg.LogFormat, os.Stderr)

	// Create and configure the MCP server
	mcpServer := server.NewNeo4jMCPServer(Version, cfg)

	// Gracefully handle shutdown
	defer func() {
		if err := mcpServer.Stop(); err != nil {
			slog.Error("Error stopping server", "error", err)
		}
	}()

	// Start the server (this blocks until the server is stopped)
	if err := mcpServer.Start(); err != nil {
		slog.Error("Server error", "error", err)
		return // so that defer can run
	}

}
