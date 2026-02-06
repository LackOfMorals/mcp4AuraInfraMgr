// This package implements a three-tool pattern for MCP operations:
//
//  1. **list-outcome** - Lists all available operations
//  2. **get-outcome-details** - Gets detailed information about a specific operation
//  3. **execute-outcome** - Executes the operation
//
//  An outcome provides the desired end state using any supplied parameter. Outcome is used to
//  differentiate between MCP Tool ( as there are only three MCP tools as described above ).
//
// tools_handlers.go holds the MCP Tool handlers for the three-tool pattern

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ListOutcomesHandler returns a handler function for listing all available Outcomes
func ListOutcomesHandler(deps *Dependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		summaries := deps.OutComes.GetAllSummaries()

		jsonData, err := json.MarshalIndent(summaries, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize Outcomes: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}

// GetOutcomeDetailsHandler returns a handler function for getting Outcome details
func GetOutcomeDetailsHandler(deps *Dependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Type assert Arguments to map[string]interface{}
		arguments, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments format"), nil
		}

		// Extract Outcome_id from request
		OutcomeID, ok := arguments["Outcome_id"].(string)
		if !ok || OutcomeID == "" {
			return mcp.NewToolResultError("Outcome_id parameter is required and must be a string"), nil
		}

		// Get the Outcome details
		Outcome, err := deps.OutComes.GetOutcome(OutcomeID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonData, err := json.MarshalIndent(Outcome, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize Outcome details: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}

// ExecuteOutcomeHandler returns a handler function for executing an Outcome
func ExecuteOutcomeHandler(deps *Dependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Type assert Arguments to map[string]interface{}
		arguments, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments format"), nil
		}

		// Extract Outcome_id from request
		OutcomeID, ok := arguments["Outcome_id"].(string)
		if !ok || OutcomeID == "" {
			return mcp.NewToolResultError("Outcome_id parameter is required and must be a string"), nil
		}

		// Extract parameters (optional, defaults to empty map)
		var parameters map[string]interface{}
		if paramsVal, exists := arguments["parameters"]; exists {
			if params, ok := paramsVal.(map[string]interface{}); ok {
				parameters = params
			} else {
				return mcp.NewToolResultError("parameters must be an object/map"), nil
			}
		} else {
			parameters = make(map[string]interface{})
		}

		// Execute the Outcome
		return deps.OutComes.ExecuteOutcome(ctx, OutcomeID, parameters, deps)
	}
}
