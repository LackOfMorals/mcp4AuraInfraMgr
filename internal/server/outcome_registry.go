// This

package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// OutcomeRegistry manages all available Outcomes
type OutcomeRegistry struct {
	Outcomes map[string]*Outcome
}

// NewOutcomeRegistry creates a new Outcome registry with all available Outcomes
func NewOutcomeRegistry() *OutcomeRegistry {
	registry := &OutcomeRegistry{
		Outcomes: make(map[string]*Outcome),
	}

	// Register all available Outcomes
	// Add more Outcomes here as they are developed
	registry.registerListInstancesOutcome()
	registry.registerGetInstanceDetailsOutcome()
	registry.registerCreateInstanceOutcome()
	registry.registerDeleteInstanceOutcome()

	return registry
}

// GetAllSummaries returns summaries of all Outcomes
func (r *OutcomeRegistry) GetAllSummaries() []OutcomeSummary {
	summaries := make([]OutcomeSummary, 0, len(r.Outcomes))
	for _, Outcome := range r.Outcomes {
		summaries = append(summaries, OutcomeSummary{
			ID:          Outcome.ID,
			Name:        Outcome.Name,
			Description: Outcome.Description,
			Type:        Outcome.Type,
			ReadOnly:    Outcome.ReadOnly,
		})
	}
	return summaries
}

// GetOutcome returns the full details of a specific Outcome
func (r *OutcomeRegistry) GetOutcome(id string) (*Outcome, error) {
	Outcome, exists := r.Outcomes[id]
	if !exists {
		return nil, fmt.Errorf("Outcome with ID '%s' not found", id)
	}
	return Outcome, nil
}

// ExecuteOutcome executes a specific Outcome with provided parameters
func (r *OutcomeRegistry) ExecuteOutcome(ctx context.Context, id string, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	Outcome, err := r.GetOutcome(id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Check if this is a write operation and we're in read-only mode
	if !Outcome.ReadOnly && deps.Config != nil && deps.Config.ReadOnly {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Cannot execute '%s' Outcome: server is in read-only mode. Write operations are disabled. Set READ_ONLY=false to enable write operations.",
			id,
		)), nil
	}

	// Execute the handler associated with this Outcome
	if Outcome.Handler == nil {
		return mcp.NewToolResultError(fmt.Sprintf("no handler registered for Outcome: %s", id)), nil
	}

	return Outcome.Handler(ctx, parameters, deps)
}
