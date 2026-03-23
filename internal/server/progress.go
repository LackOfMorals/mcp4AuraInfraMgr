// progress.go
//
// Helpers for sending MCP notifications/progress to the client during
// long-running outcome handlers (e.g. provision-environment).
//
// The MCP spec allows a server to send out-of-band progress notifications
// while a tool call is still in-flight.  The client attaches a progressToken
// in _meta when it wants these; we check for it and fire if present.
//
// If the client did not supply a token (older clients, or clients that opted
// out) the helpers are no-ops — the outcome still works, just silently.

package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// progressSender holds the context and server reference needed to send
// notifications/progress, plus the token that ties them to the original request.
type progressSender struct {
	ctx    context.Context
	srv    *server.MCPServer
	token  mcp.ProgressToken
	active bool // false when no token was provided
}

// newProgressSender constructs a sender from the tool call request.
// If the client did not include a progressToken the sender is inert.
func newProgressSender(ctx context.Context, srv *server.MCPServer, req mcp.CallToolRequest) *progressSender {
	ps := &progressSender{ctx: ctx, srv: srv}

	if req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
		ps.token = req.Params.Meta.ProgressToken
		ps.active = true
	}

	return ps
}

// Send fires a notifications/progress message.
// progress is 0.0–1.0 for determinate progress, or -1 for indeterminate.
// message is a human-readable status line for the client/LLM to display.
func (ps *progressSender) Send(progress float64, message string) {
	if !ps.active || ps.srv == nil {
		return
	}

	// SendNotificationToClient is best-effort; never fail the outcome over it.
	_ = ps.srv.SendNotificationToClient(ps.ctx, "notifications/progress", map[string]any{
		"progressToken": ps.token,
		"progress":      progress,
		"message":       message,
	})
}

// Statusf formats a message and sends with an indeterminate progress value (-1).
func (ps *progressSender) Statusf(format string, args ...any) {
	ps.Send(-1, fmt.Sprintf(format, args...))
}

// Step sends a discrete step out of a known total (e.g. step 2 of 4).
func (ps *progressSender) Step(current, total int, message string) {
	var pct float64
	if total > 0 {
		pct = float64(current) / float64(total)
	}
	ps.Send(pct, fmt.Sprintf("[%d/%d] %s", current, total, message))
}
