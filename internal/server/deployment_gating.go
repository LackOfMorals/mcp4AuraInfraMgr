// deployment_gating.go
//
// Implements two complementary workflow outcomes for safe deployments:
//
//  1. pre-deployment-snapshot — Take an on-demand snapshot of an instance and
//     poll until it reaches "Completed" status.  Returns a rollback handle
//     (instance_id + snapshot_id + timestamp) that can be passed directly to
//     rollback-instance if a subsequent migration goes wrong.
//
//  2. rollback-instance — Restore an instance to a specific snapshot.  Validates
//     the snapshot is completed and exportable before proceeding, requires explicit
//     confirmation, and polls until the instance is running again.
//
// Both outcomes send progress notifications on every poll tick so the MCP client
// receives live status updates.  They follow the same pattern as provision-environment:
// a public stub registered in the outcome registry delegates to a WithProgress variant
// that is called directly from ExecuteOutcomeHandler with a live progressSender.
//
// These two outcomes are designed to be used together as a migration safety pattern:
//
//   pre-deployment-snapshot → run migration → (if bad) rollback-instance

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	snapshotPollInterval   = 10 * time.Second
	snapshotTimeoutMinutes = 30.0
	restoreTimeoutMinutes  = 20.0
)

// ── pre-deployment-snapshot ───────────────────────────────────────────────────

func (r *OutcomeRegistry) registerPreDeploymentSnapshotOutcome() {
	r.Outcomes["pre-deployment-snapshot"] = &Outcome{
		ID:   "pre-deployment-snapshot",
		Name: "Pre-Deployment Snapshot",
		Description: `Take an on-demand snapshot of an instance and wait until it is fully completed.
Returns a rollback handle containing the instance_id and snapshot_id that can be
passed directly to 'rollback-instance' if a subsequent migration fails.

Unlike 'snapshot-instance' (which returns immediately), this outcome blocks until
the snapshot status is 'Completed' and confirms it is exportable — making it safe
to use as a deployment gate before running schema changes or data migrations.

Progress notifications are sent during the snapshot so clients can display live status.`,
		Type:     OutcomesTypeCreate,
		ReadOnly: false,
		Parameters: []OutcomeParameter{
			{
				Name:        "instance_id",
				Type:        "string",
				Description: "The ID of the instance to snapshot before deployment.",
				Required:    true,
			},
			{
				Name:        "deployment_label",
				Type:        "string",
				Description: "Optional: a human-readable label describing what deployment this checkpoint is for (e.g. 'v2.3-schema-migration'). Included in the rollback handle for traceability.",
				Required:    false,
			},
		},
		Metadata: map[string]interface{}{
			"category":       "deployment",
			"workflow_steps": []string{"create-snapshot", "poll-until-completed", "return-rollback-handle"},
		},
		Handler: executePreDeploymentSnapshot,
	}
}

// executePreDeploymentSnapshot is the outcome Handler registered in the registry.
// Delegates to the WithProgress variant with a nil sender (no notifications).
// When called via ExecuteOutcomeHandler the WithProgress variant is called directly
// with a live progressSender built from the request's progressToken.
func executePreDeploymentSnapshot(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	return executePreDeploymentSnapshotWithProgress(ctx, parameters, deps, nil)
}

// rollbackHandle is the JSON payload returned by pre-deployment-snapshot and
// consumed as input by rollback-instance.
type rollbackHandle struct {
	InstanceID      string `json:"instance_id"`
	SnapshotID      string `json:"snapshot_id"`
	Timestamp       string `json:"timestamp"`
	Exportable      bool   `json:"exportable"`
	DeploymentLabel string `json:"deployment_label,omitempty"`
	CreatedAt       string `json:"created_at"`
}

type preDeploymentSnapshotResult struct {
	Success        bool           `json:"success"`
	Message        string         `json:"message"`
	RollbackHandle rollbackHandle `json:"rollback_handle"`
	Warning        string         `json:"warning,omitempty"`
}

func executePreDeploymentSnapshotWithProgress(
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

	instanceID, ok := parameters["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("'instance_id' is required and must be a non-empty string"), nil
	}

	deploymentLabel, _ := parameters["deployment_label"].(string)

	// ── 1. Verify instance is running before snapshotting ────────────────────

	sendStatus(fmt.Sprintf("Checking instance '%s' status…", instanceID))

	instanceInfo, err := deps.AClient.Instances.Get(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve instance details: %v", err)), nil
	}
	if instanceInfo.Data.Status != "running" {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Instance '%s' is in status '%s' — must be 'running' before a snapshot can be taken.",
			instanceID, instanceInfo.Data.Status,
		)), nil
	}

	// ── 2. Trigger on-demand snapshot ────────────────────────────────────────

	labelMsg := ""
	if deploymentLabel != "" {
		labelMsg = fmt.Sprintf(" (label: %s)", deploymentLabel)
	}
	sendStatus(fmt.Sprintf("Creating pre-deployment snapshot for instance '%s'%s…", instanceID, labelMsg))

	snapshotResp, err := deps.AClient.Snapshots.Create(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create snapshot for instance '%s': %v", instanceID, err)), nil
	}
	snapshotID := snapshotResp.Data.SnapshotID

	sendStatus(fmt.Sprintf("Snapshot '%s' created. Waiting for it to complete…", snapshotID))

	// ── 3. Poll until Completed ───────────────────────────────────────────────

	timeout := time.Duration(snapshotTimeoutMinutes) * time.Minute
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(snapshotPollInterval)
	defer ticker.Stop()

	var lastStatus string
	var finalTimestamp string
	var finalExportable bool
	elapsed := 0

	for {
		select {
		case <-pollCtx.Done():
			return mcp.NewToolResultError(fmt.Sprintf(
				"Snapshot '%s' for instance '%s' did not complete within %.0f minutes. "+
					"Last status: '%s'. Use 'list-snapshot-instance' to check its current state.",
				snapshotID, instanceID, snapshotTimeoutMinutes, lastStatus,
			)), nil

		case <-ticker.C:
			elapsed += int(snapshotPollInterval.Seconds())

			details, err := deps.AClient.Snapshots.Get(pollCtx, instanceID, snapshotID)
			if err != nil {
				lastStatus = fmt.Sprintf("poll-error: %v", err)
				sendStatus(fmt.Sprintf("Snapshot '%s' — status check failed (will retry): %v", snapshotID, err))
				continue
			}

			lastStatus = details.Data.Status
			finalTimestamp = details.Data.Timestamp
			finalExportable = details.Data.Exportable

			sendStatus(fmt.Sprintf(
				"Snapshot '%s' — status: %s, exportable: %v (elapsed: %ds / timeout: %.0fm)",
				snapshotID, lastStatus, finalExportable, elapsed, snapshotTimeoutMinutes,
			))

			// Aura API returns "Completed" (capital C). Guard against case variations.
			if strings.EqualFold(lastStatus, "completed") {
				if progress != nil {
					progress.Send(1.0, fmt.Sprintf("Snapshot '%s' completed. Rollback handle ready.", snapshotID))
				}

				handle := rollbackHandle{
					InstanceID:      instanceID,
					SnapshotID:      snapshotID,
					Timestamp:       finalTimestamp,
					Exportable:      finalExportable,
					DeploymentLabel: deploymentLabel,
					CreatedAt:       time.Now().UTC().Format(time.RFC3339),
				}

				warning := ""
				if !finalExportable {
					warning = "Snapshot is not yet marked as exportable. " +
						"Rollback via 'rollback-instance' may not be available immediately — " +
						"check exportable status before proceeding with a risky migration."
				}

				result := preDeploymentSnapshotResult{
					Success:        true,
					Message:        fmt.Sprintf("Snapshot completed for instance '%s'. Rollback handle is ready.", instanceID),
					RollbackHandle: handle,
					Warning:        warning,
				}

				jsonData, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Snapshot completed but failed to serialize result: %v", err)), nil
				}
				return mcp.NewToolResultText(string(jsonData)), nil
			}

			// Any explicit failure status — bail immediately.
			if strings.EqualFold(lastStatus, "failed") || strings.EqualFold(lastStatus, "error") {
				return mcp.NewToolResultError(fmt.Sprintf(
					"Snapshot '%s' for instance '%s' entered status '%s'. "+
						"Use 'list-snapshot-instance' to investigate.",
					snapshotID, instanceID, lastStatus,
				)), nil
			}

			// Still in progress — keep polling.
		}
	}
}

// ── rollback-instance ─────────────────────────────────────────────────────────

func (r *OutcomeRegistry) registerRollbackInstanceOutcome() {
	r.Outcomes["rollback-instance"] = &Outcome{
		ID:   "rollback-instance",
		Name: "Rollback Instance",
		Description: `Restore an instance to a specific snapshot taken by 'pre-deployment-snapshot'.
Validates that the snapshot is completed and exportable before proceeding.
Requires explicit confirmation as this operation overwrites ALL current data in the instance.
Polls until the instance is running again and returns the restored state.

Progress notifications are sent during the restore so clients can display live status.

Use the rollback_handle returned by 'pre-deployment-snapshot' to populate instance_id
and snapshot_id.`,
		Type:     OutcomesTypeUpdate,
		ReadOnly: false,
		Parameters: []OutcomeParameter{
			{
				Name:        "instance_id",
				Type:        "string",
				Description: "The ID of the instance to restore. Use the instance_id from the rollback_handle.",
				Required:    true,
			},
			{
				Name:        "snapshot_id",
				Type:        "string",
				Description: "The ID of the snapshot to restore from. Use the snapshot_id from the rollback_handle.",
				Required:    true,
			},
			{
				Name:        "confirm",
				Type:        "boolean",
				Description: "Must be true to confirm the rollback. WARNING: this overwrites ALL current data in the instance with the snapshot data.",
				Required:    true,
			},
		},
		Metadata: map[string]interface{}{
			"category":       "deployment",
			"destructive":    true,
			"warning":        "Restoring a snapshot overwrites ALL current data in the instance. This cannot be undone.",
			"workflow_steps": []string{"validate-snapshot", "restore", "poll-until-running"},
		},
		Handler: executeRollbackInstance,
	}
}

// executeRollbackInstance is the outcome Handler registered in the registry.
// Delegates to the WithProgress variant with a nil sender (no notifications).
// When called via ExecuteOutcomeHandler the WithProgress variant is called directly
// with a live progressSender built from the request's progressToken.
func executeRollbackInstance(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	return executeRollbackInstanceWithProgress(ctx, parameters, deps, nil)
}

type rollbackResult struct {
	Success              bool   `json:"success"`
	Message              string `json:"message"`
	InstanceID           string `json:"instance_id"`
	InstanceName         string `json:"instance_name"`
	Status               string `json:"status"`
	RestoredFromSnapshot string `json:"restored_from_snapshot"`
	RestoredAt           string `json:"restored_at"`
}

func executeRollbackInstanceWithProgress(
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

	// ── 1. Extract and validate parameters ───────────────────────────────────

	instanceID, ok := parameters["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("'instance_id' is required"), nil
	}

	snapshotID, ok := parameters["snapshot_id"].(string)
	if !ok || snapshotID == "" {
		return mcp.NewToolResultError("'snapshot_id' is required. Use the snapshot_id from the rollback_handle returned by 'pre-deployment-snapshot'"), nil
	}

	confirm, ok := parameters["confirm"].(bool)
	if !ok || !confirm {
		return mcp.NewToolResultError(
			"'confirm' must be set to true to proceed. " +
				"WARNING: rollback overwrites ALL current instance data with the snapshot. This cannot be undone.",
		), nil
	}

	// ── 2. Validate the snapshot is usable ───────────────────────────────────

	sendStatus(fmt.Sprintf("Validating snapshot '%s' for instance '%s'…", snapshotID, instanceID))

	snapshotDetails, err := deps.AClient.Snapshots.Get(ctx, instanceID, snapshotID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Failed to retrieve snapshot '%s' for instance '%s': %v. "+
				"Use 'list-snapshot-instance' to confirm the snapshot exists.",
			snapshotID, instanceID, err,
		)), nil
	}

	if !strings.EqualFold(snapshotDetails.Data.Status, "completed") {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Snapshot '%s' has status '%s' — only 'Completed' snapshots can be used for rollback.",
			snapshotID, snapshotDetails.Data.Status,
		)), nil
	}

	if !snapshotDetails.Data.Exportable {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Snapshot '%s' is not exportable and cannot be used for rollback. "+
				"This can happen with differential backups or snapshots outside the retention window. "+
				"Use 'list-snapshot-instance' to find an exportable snapshot.",
			snapshotID,
		)), nil
	}

	// ── 3. Get instance name for result messaging ─────────────────────────────

	instanceInfo, err := deps.AClient.Instances.Get(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve instance details: %v", err)), nil
	}
	instanceName := instanceInfo.Data.Name

	// ── 4. Trigger the restore ────────────────────────────────────────────────

	sendStatus(fmt.Sprintf("Initiating rollback of instance '%s' to snapshot '%s'…", instanceName, snapshotID))

	_, err = deps.AClient.Snapshots.Restore(ctx, instanceID, snapshotID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Failed to initiate rollback of instance '%s' to snapshot '%s': %v",
			instanceID, snapshotID, err,
		)), nil
	}

	restoredAt := time.Now().UTC().Format(time.RFC3339)

	sendStatus(fmt.Sprintf("Rollback initiated for instance '%s'. Waiting for it to return to running…", instanceName))

	// ── 5. Poll until running ─────────────────────────────────────────────────
	// During restore the instance transitions through: restoring → paused → running
	// (exact intermediate states vary by tier).

	timeout := time.Duration(restoreTimeoutMinutes) * time.Minute
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(snapshotPollInterval)
	defer ticker.Stop()

	var lastStatus string
	elapsed := 0

	for {
		select {
		case <-pollCtx.Done():
			return mcp.NewToolResultError(fmt.Sprintf(
				"Instance '%s' (ID: %s) did not return to 'running' within %.0f minutes after rollback. "+
					"Last status: '%s'. Use 'get-instance-details' to monitor its current state.",
				instanceName, instanceID, restoreTimeoutMinutes, lastStatus,
			)), nil

		case <-ticker.C:
			elapsed += int(snapshotPollInterval.Seconds())

			details, err := deps.AClient.Instances.Get(pollCtx, instanceID)
			if err != nil {
				lastStatus = fmt.Sprintf("poll-error: %v", err)
				sendStatus(fmt.Sprintf("Waiting for '%s' — status check failed (will retry): %v", instanceName, err))
				continue
			}

			lastStatus = details.Data.Status

			sendStatus(fmt.Sprintf(
				"Instance '%s' (ID: %s) — status: %s (elapsed: %ds / timeout: %.0fm)",
				instanceName, instanceID, lastStatus, elapsed, restoreTimeoutMinutes,
			))

			if lastStatus == "running" {
				if progress != nil {
					progress.Send(1.0, fmt.Sprintf("Instance '%s' is running. Rollback complete.", instanceName))
				}

				result := rollbackResult{
					Success:              true,
					Message:              fmt.Sprintf("Instance '%s' has been successfully rolled back and is running.", instanceName),
					InstanceID:           instanceID,
					InstanceName:         instanceName,
					Status:               lastStatus,
					RestoredFromSnapshot: snapshotID,
					RestoredAt:           restoredAt,
				}

				jsonData, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Rollback succeeded but failed to serialize result: %v", err)), nil
				}
				return mcp.NewToolResultText(string(jsonData)), nil
			}

			if lastStatus == "error" {
				return mcp.NewToolResultError(fmt.Sprintf(
					"Instance '%s' entered error state after rollback. "+
						"Use 'get-instance-details' with instance_id '%s' to investigate.",
					instanceName, instanceID,
				)), nil
			}

			// restoring, loading, paused (transitional) — keep waiting.
		}
	}
}
