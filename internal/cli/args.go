package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// osExit is a variable that can be mocked in tests
var osExit = os.Exit

const helpText = `mcp-aura-api - A Model Context Protocol Server for Neo4j Aura API

Usage:
  mcp-aura-api  [OPTIONS]

Options:
  -h, --help                          Show this help message
  -v, --version                       Show version information
  

Required Environment Variables:
  CLIENT_ID       Client Id 
  CLIENT_SECRET   Client Secret
  
Optional Environment Variables:
  URI             URI to Aura API 
  READ_ONLY       Enable read-only mode (default: true)
  LOG_LEVEL       Log level to use (default: Info )
  LOG_FORMAT      Log format to use (defaut: Text )

Examples:
  # Using environment variables

  # Using CLI flags (takes precedence over environment variables)
  mcp-aura-api --client-id <YOUR CLIENT ID> --client-secret <YOUR CLIENT SECRET>

For more information, visit: https://github.com/neo4j/mcp
`

// Args holds configuration values parsed from command-line flags
type Args struct {
	URI          string
	ClientId     string
	ClientSecret string
	ReadOnly     string
	LogLevel     string
	LogFormat    string
}

// ParseConfigFlags parses CLI flags and returns configuration values.
// It should be called after HandleArgs to ensure help/version flags are processed first.
func ParseConfigFlags() *Args {
	URI := flag.String("uri", "", "Neo4j Aura API URI (overrides URI env var)")
	ReadOnly := flag.String("read-only", "", "Enable read-only mode: true or false (overrides READ_ONLY env var)")
	ClientId := flag.String("client-id", "", "Client Id for Aura API ")
	ClientSecret := flag.String("client-secret", "", "Client Secret for Aura API ")
	LogLevel := flag.String("log-level", "", "Log level to use ( overrides LOG_LEVEL )")
	LogFormat := flag.String("log-format", "", "Log level to use ( overrides LOG_FORMAT )")

	flag.Parse()

	return &Args{
		URI:          *URI,
		ReadOnly:     *ReadOnly,
		ClientId:     *ClientId,
		ClientSecret: *ClientSecret,
		LogLevel:     *LogLevel,
		LogFormat:    *LogFormat,
	}
}

// HandleArgs processes command-line arguments for version and help flags.
// It exits the program after displaying the requested information.
// If unknown flags are encountered, it prints an error message and exits.
// Known configuration flags are skipped here so that the flag package in main.go can handle them properly.
func HandleArgs(version string) {
	if len(os.Args) <= 1 {
		return
	}

	flags := make(map[string]bool)
	var err error
	i := 1 // we start from 1 because os.Args[0] is the program name ("mcp-aura-api") - not a flag

	for i < len(os.Args) {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			flags["help"] = true
			i++
		case "-v", "--version":
			flags["version"] = true
			i++
		// Allow configuration flags to be parsed by the flag package
		case "--uri", "--read-only", "--client-id", "--client-secret", "--log-level", "--log-format":
			// Check if there's a value following the flag
			if i+1 >= len(os.Args) {
				err = fmt.Errorf("%s requires a value", arg)
				break
			}
			// Check if next argument is another flag (starts with --)
			nextArg := os.Args[i+1]
			if strings.HasPrefix(nextArg, "-") {
				err = fmt.Errorf("%s requires a value (got flag %s instead)", arg, nextArg)
				break
			}
			// Safe to skip flag and value - let flag package handle them
			i += 2
		default:
			if arg == "--" {
				// Stop processing our flags, let flag package handle the rest
				i = len(os.Args)
			} else {
				err = fmt.Errorf("unknown flag or argument: %s", arg)
				i++
			}
		}
		// Exit loop if an error occurred
		if err != nil {
			break
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		osExit(1)
	}

	if flags["help"] {
		fmt.Print(helpText)
		osExit(0)
	}

	if flags["version"] {
		fmt.Printf("neo4j-mcp version: %s\n", version)
		osExit(0)
	}
}
