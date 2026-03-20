// environment_lifecycle.go
//
// Implements the "provision-environment" outcome — a workflow outcome that
// creates an Aura instance from a named configuration and blocks until the
// instance reaches "running" status (or a terminal error / timeout occurs).
//
// Progress notifications are sent on every poll tick so the MCP client (and
// the LLM/user watching it) gets real-time status updates without needing to
// poll the MCP server themselves.
//
// This is distinct from create-instance / create-instance-from-config, which
// fire-and-forget. Here we own the async polling loop so the LLM (or CI
// pipeline) receives a single, actionable result: connection URL + credentials,
// ready to use.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultPollInterval   = 10 * time.Second
	defaultTimeoutMinutes = 15.0
)

// registerProvisionEnvironmentOutcome registers the provision-environment outcome.
func (r *OutcomeRegistry) registerProvisionEnvironmentOutcome() {
	r.Outcomes["provision-environment"] = &Outcome{
		ID:   "provision-environment",
		Name: "Provision Environment",
		Description: `Create a new Neo4j Aura instance from a named configuration template and wait
until it is fully running. Returns the connection URL, credentials, and instance
ID in a single response — no polling required from the caller.

Progress notifications are sent during provisioning so clients can display
live status (creating → loading → running).

Use 'list-instance-configs' to discover available configuration labels.
Use 'instance_name' to override the default name (e.g. for a feature branch).`,
		Type:     OutcomesTypeCreate,
		ReadOnly: false,
		Parameters: []OutcomeParameter{
			{
				Name:        "config_label",
				Type:        "string",
				Description: "Label of the pre-configured instance template. Use 'list-instance-configs' to see available labels.",
				Required:    true,
			},
			{
				Name:        "instance_name",
				Type:        "string",
				Description: "Optional: override the default name from the configuration (e.g. 'feature-branch-xyz', 'staging-load-test').",
				Required:    false,
			},
			{
				Name:        "wait_timeout_minutes",
				Type:        "number",
				Description: "Maximum minutes to wait for the instance to become ready. Defaults to 15.",
				Required:    false,
				Default:     defaultTimeoutMinutes,
			},
		},
		Metadata: map[string]interface{}{
			"category":       "lifecycle",
			"workflow_steps": []string{"validate-config", "create-instance", "poll-until-running"},
			"warning":        "Provisions a new billable Aura instance.",
		},
		Handler: executeProvisionEnvironment,
	}
}

// provisionResult is the JSON payload returned on success.
type provisionResult struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	InstanceID    string `json:"instance_id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	ConnectionURL string `json:"connection_url"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	CloudProvider string `json:"cloud_provider"`
	Region        string `json:"region"`
	Memory        string `json:"memory"`
	Type          string `json:"type"`
	ConfigUsed    string `json:"config_used"`
}

// executeProvisionEnvironment is the outcome Handler registered in the registry.
// It delegates to executeProvisionEnvironmentWithProgress with a nil progressSender,
// which means no progress notifications are sent via this path.
//
// When called via ExecuteOutcomeHandler in tool_handlers.go, the handler is
// bypassed and executeProvisionEnvironmentWithProgress is called directly with
// a live progressSender built from the request's progressToken.
func executeProvisionEnvironment(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	return executeProvisionEnvironmentWithProgress(ctx, parameters, deps, nil)
}

// executeProvisionEnvironmentWithProgress is the real implementation.
// progress may be nil — all progress.Send* calls are no-ops in that case.
func executeProvisionEnvironmentWithProgress(
	ctx context.Context,
	parameters map[string]interface{},
	deps *Dependencies,
	progress *progressSender,
) (*mcp.CallToolResult, error) {

	if deps.AClient == nil {
		return mcp.NewToolResultError("Aura API Client is not initialized"), nil
	}

	sendStatus := func(msg string) {
		if progress != nil {
			progress.Statusf("%s", msg)
		}
	}

	// ── 1. Validate & resolve the configuration ──────────────────────────────

	sendStatus("Validating configuration…")

	configLabel, ok := parameters["config_label"].(string)
	if !ok || configLabel == "" {
		return mcp.NewToolResultError("'config_label' is required. Use 'list-instance-configs' to see available labels."), nil
	}

	cfgFile, err := LoadInstanceConfigurations(deps.Config.InstanceCfgFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load instance configurations: %v", err)), nil
	}

	cfg, err := cfgFile.GetConfigByLabel(configLabel)
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("Configuration '%s' not found: %v. Use 'list-instance-configs' to see available labels.", configLabel, err),
		), nil
	}

	if err := cfg.ValidateConfig(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Configuration '%s' is invalid: %v", configLabel, err)), nil
	}

	// ── 2. Resolve instance name (optional override) ──────────────────────────

	instanceName := cfg.Name
	if override, ok := parameters["instance_name"].(string); ok && override != "" {
		instanceName = override
	}

	// ── 3. Resolve timeout ────────────────────────────────────────────────────

	timeoutMinutes := defaultTimeoutMinutes
	if tm, ok := parameters["wait_timeout_minutes"].(float64); ok && tm > 0 {
		timeoutMinutes = tm
	}
	timeout := time.Duration(timeoutMinutes) * time.Minute

	// ── 4. Create the instance ────────────────────────────────────────────────

	sendStatus(fmt.Sprintf("Creating instance '%s' using config '%s'…", instanceName, configLabel))

	instanceDef := cfg.ToCreateInstanceConfig()
	instanceDef.Name = instanceName

	created, err := deps.AClient.Instances.Create(ctx, instanceDef)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create instance '%s': %v", instanceName, err)), nil
	}

	instanceID := created.Data.ID
	// Credentials are only returned at creation time — capture them before polling.
	username := created.Data.Username
	password := created.Data.Password

	sendStatus(fmt.Sprintf("Instance '%s' created (ID: %s). Waiting for it to become ready…", instanceName, instanceID))

	// ── 5. Poll until running ─────────────────────────────────────────────────
	// Aura instance status path: creating → loading → running
	// Terminal error state: "error"

	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	var lastStatus string
	elapsed := 0

	for {
		select {
		case <-pollCtx.Done():
			return mcp.NewToolResultError(fmt.Sprintf(
				"Instance '%s' (ID: %s) did not reach 'running' state within %.0f minutes. "+
					"Last observed status: '%s'. "+
					"The instance may still be provisioning — use 'get-instance-details' with instance_id '%s' to check.",
				instanceName, instanceID, timeoutMinutes, lastStatus, instanceID,
			)), nil

		case <-ticker.C:
			elapsed += int(defaultPollInterval.Seconds())

			details, err := deps.AClient.Instances.Get(pollCtx, instanceID)
			if err != nil {
				// Transient API error — log via progress and keep polling until timeout.
				lastStatus = fmt.Sprintf("poll-error: %v", err)
				sendStatus(fmt.Sprintf("Waiting for '%s' — status check failed (will retry): %v", instanceName, err))
				continue
			}

			lastStatus = details.Data.Status

			sendStatus(fmt.Sprintf(
				"Instance '%s' (ID: %s) — status: %s (elapsed: %ds / timeout: %.0fm)",
				instanceName, instanceID, lastStatus, elapsed, timeoutMinutes,
			))

			switch lastStatus {
			case "running":
				if progress != nil {
					progress.Send(1.0, fmt.Sprintf("Instance '%s' is running and ready.", instanceName))
				}

				result := provisionResult{
					Success:       true,
					Message:       fmt.Sprintf("Instance '%s' is running and ready to use.", instanceName),
					InstanceID:    instanceID,
					Name:          details.Data.Name,
					Status:        details.Data.Status,
					ConnectionURL: details.Data.ConnectionUrl,
					Username:      username,
					Password:      password,
					CloudProvider: details.Data.CloudProvider,
					Region:        details.Data.Region,
					Memory:        details.Data.Memory,
					Type:          details.Data.Type,
					ConfigUsed:    configLabel,
				}

				jsonData, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return mcp.NewToolResultError(
						fmt.Sprintf("Instance is running but failed to serialize result: %v", err),
					), nil
				}
				return mcp.NewToolResultText(string(jsonData)), nil

			case "error":
				return mcp.NewToolResultError(fmt.Sprintf(
					"Instance '%s' (ID: %s) entered error state during provisioning. "+
						"Use 'get-instance-details' with instance_id '%s' to investigate.",
					instanceName, instanceID, instanceID,
				)), nil

				// All other statuses (creating, loading, updating, …) → keep waiting.
			}
		}
	}
}
