package server

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// OutcomesType represents the type/category of an Outcomes
type OutcomesType string

const (
	OutcomesTypeList   OutcomesType = "list"
	OutcomesTypeRead   OutcomesType = "read"
	OutcomesTypeCreate OutcomesType = "create"
	OutcomesTypeUpdate OutcomesType = "update"
	OutcomesTypeDelete OutcomesType = "delete"
)

// OutcomesHandler is a function that executes an Outcomes
type OutcomesHandler func(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error)

// Outcome represents a high-level operation that can be performed
type Outcome struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        OutcomesType           `json:"type"`
	ReadOnly    bool                   `json:"readonly"`
	Parameters  []OutcomeParameter     `json:"parameters,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Handler     OutcomesHandler        `json:"-"` // Handler function (not serialized to JSON)
}

// OutcomesParameter represents a parameter required for an Outcomes
type OutcomeParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// OutcomesSummary is a lightweight version for listing Outcomess
type OutcomeSummary struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        OutcomesType `json:"type"`
	ReadOnly    bool         `json:"readonly"`
}
