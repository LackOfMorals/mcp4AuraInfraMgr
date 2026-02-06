// This package implements a three-tool pattern for MCP operations:
//
//  1. **list-outcome** - Lists all available operations
//  2. **get-outcome-details** - Gets detailed information about a specific operation
//  3. **execute-outcome** - Executes the operation
//
//  An outcome provides the desired end state using any supplied parameter. Outcome is used to
//  differentiate between MCP Tool ( as there are only three MCP tools as described above ).
//
// tools_spec.go holds the MCP Tool spec for the three-tool pattern

package server

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// ListOutcomesSpec returns the tool specification for listing available Tools
func ListOutcomesSpec() mcp.Tool {
	return mcp.NewTool("list-outcomes",
		mcp.WithDescription(`List all available outcomes (operations) that can be performed on Neo4j Aura resources.
Returns a summary of each outcomes including its ID, name, description, type, and whether it's read-only.
Use this to discover what operations are available before getting details or executing them.`),
		mcp.WithTitleAnnotation("List Available Outcomes"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	)
}

// GetOutcomeDetailsSpec returns the outcome specification for getting outcome details
func GetOutcomeDetailsSpec() mcp.Tool {
	return mcp.NewTool("get-outcome-details",
		mcp.WithDescription(`Get detailed information about a specific outcome including its full parameters and requirements.
Use this after 'list-outcomes' to understand what parameters are needed before executing an outcome.`),
		mcp.WithTitleAnnotation("Get Outcome Details"),
		mcp.WithString("Outcome_id",
			mcp.Required(),
			mcp.Description("The ID of the outcome to get details for (from list-outcomes)")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	)
}

// ExecuteOutcomeSpec returns the tool specification for executing an outcome.
func ExecuteOutcomeSpec() mcp.Tool {
	return mcp.NewTool("execute-outcome",
		mcp.WithDescription(`Execute a specific outcome with the provided parameters.
Use 'list-outcomes' to see available Tools and 'get-outcome-details' to understand required parameters.`),
		mcp.WithTitleAnnotation("Execute outcome"),
		mcp.WithString("Outcome_id",
			mcp.Required(),
			mcp.Description("The ID of the outcome to execute (from list-outcomes)")),
		mcp.WithObject("parameters",
			mcp.Description("Parameters required for the outcome (see get-outcome-details for parameter specifications)"),
			mcp.AdditionalProperties(true)),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false), // Will vary by outcome
		mcp.WithOpenWorldHintAnnotation(false),
	)
}
