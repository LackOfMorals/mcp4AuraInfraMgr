// =============================================================================
// These are all of the instance related outcomes
// =============================================================================

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LackOfMorals/aura-client"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerListInstancesOutcome registers the list-instances Outcome
func (r *OutcomeRegistry) registerListInstancesOutcome() {
	r.Outcomes["list-instances"] = &Outcome{
		ID:          "list-instances",
		Name:        "List Instances",
		Description: "Retrieve a list of all Neo4j Aura database instances. Returns instance details including name, ID, status, cloud provider, memory size, type, and connection URL.",
		Type:        OutcomesTypeList,
		ReadOnly:    true,
		Parameters:  []OutcomeParameter{}, // No parameters needed for listing
		Metadata: map[string]interface{}{
			"category": "instances",
		},
		Handler: executeListInstances,
	}
}

// executeListInstances implements the list-instances Outcome
func executeListInstances(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {

	type instanceSummary aura.ListInstanceData

	type instanceList []instanceSummary

	if deps.AClient == nil {
		return mcp.NewToolResultError("Aura API Client is not initialized"), nil
	}

	// Get the list of instances
	instances, err := deps.AClient.Instances.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list instances: %v", err)), nil
	}

	// Create an empty list
	records := instanceList{}

	// Fill the list with our instance summary
	for _, inst := range instances.Data {
		records = append(records, instanceSummary{
			Name:          inst.Name,
			Id:            inst.Id,
			Created:       inst.Created,
			CloudProvider: inst.CloudProvider,
		})
	}

	if len(records) == 0 {
		return mcp.NewToolResultText("No instances found or user does not have access to any instances."), nil
	}

	jsonData, err := json.Marshal(records)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// registerListInstanceConfigsOutcome registers the list-instance-configs outcome
func (r *OutcomeRegistry) registerListInstanceConfigsOutcome() {
	r.Outcomes["list-instance-configs"] = &Outcome{
		ID:          "list-instance-configs",
		Name:        "List Instance Configurations",
		Description: "List all available pre-configured instance templates. Each configuration has a friendly label that can be used when creating instances, avoiding the need to specify all parameters manually.",
		Type:        OutcomesTypeList,
		ReadOnly:    true,
		Parameters:  []OutcomeParameter{}, // No parameters needed
		Metadata: map[string]interface{}{
			"category": "configurations",
		},
		Handler: executeListInstanceConfigs,
	}
}

// executeListInstanceConfigs implements the list-instance-configs outcome
func executeListInstanceConfigs(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	// Load configurations from file
	configFile, err := LoadInstanceConfigurations(deps.Config.InstanceCfgFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load instance configurations: %v, use create-instance outcome to create new instances", err)), nil
	}

	// Create summary list
	type configSummary struct {
		Label         string `json:"label"`
		Description   string `json:"description,omitempty"`
		Name          string `json:"name"`
		CloudProvider string `json:"cloud_provider"`
		Region        string `json:"region"`
		Memory        string `json:"memory"`
		Type          string `json:"type"`
		TenantId      string `json:"tenant_id"`
		Version       string `json:"version,omitempty"`
	}

	summaries := make([]configSummary, 0, len(configFile.Configurations))
	for _, config := range configFile.Configurations {
		// Validate each configuration
		if err := config.ValidateConfig(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid configuration found: %v", err)), nil
		}

		summaries = append(summaries, configSummary{
			Label:         config.Label,
			Description:   config.Description,
			Name:          config.Name,
			CloudProvider: config.CloudProvider,
			Region:        config.Region,
			Memory:        config.Memory,
			Type:          config.Type,
			TenantId:      config.TenantId,
			Version:       config.Version,
		})
	}

	jsonData, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize configurations: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// registerCreateInstanceFromConfigOutcome registers the create-instance-from-config outcome
func (r *OutcomeRegistry) registerCreateInstanceFromConfigOutcome() {
	r.Outcomes["create-instance-from-config"] = &Outcome{
		ID:          "create-instance-from-config",
		Name:        "Create Instance from Configuration",
		Description: "Create a new Neo4j Aura database instance using a pre-configured template. Simply specify the configuration label to create an instance with all the predefined settings. Use 'list-instance-configs' to see available configurations.",
		Type:        OutcomesTypeCreate,
		ReadOnly:    false,
		Parameters: []OutcomeParameter{
			{
				Name:        "config_label",
				Type:        "string",
				Description: "The label of the pre-configured instance template to use. Use 'list-instance-configs' outcome to see available labels.",
				Required:    true,
			},
			{
				Name:        "override_name",
				Type:        "string",
				Description: "Optional: Override the default name from the configuration with a custom name",
				Required:    false,
			},
		},
		Metadata: map[string]interface{}{
			"category": "instances",
		},
		Handler: executeCreateInstanceFromConfig,
	}
}

// executeCreateInstanceFromConfig implements the create-instance-from-config outcome
func executeCreateInstanceFromConfig(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	if deps.AClient == nil {
		return mcp.NewToolResultError("Aura API Client is not initialized"), nil
	}

	// Validate and extract required parameter
	configLabel, ok := parameters["config_label"].(string)
	if !ok || configLabel == "" {
		return mcp.NewToolResultError("'config_label' parameter is required and must be a non-empty string"), nil
	}

	// Load configurations from file
	configFile, err := LoadInstanceConfigurations(deps.Config.InstanceCfgFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load instance configurations: %v, use create-instance outcome to create new instances.", err)), nil
	}

	// Get the specific configuration
	config, err := configFile.GetConfigByLabel(configLabel)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Configuration not found: %v. Use 'list-instance-configs' to see available configurations.", err)), nil
	}

	// Validate the configuration
	if err := config.ValidateConfig(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid configuration: %v", err)), nil
	}

	// Check for name override
	instanceName := config.Name
	if overrideName, ok := parameters["override_name"].(string); ok && overrideName != "" {
		instanceName = overrideName
	}

	// Create the instance definition
	instanceDefinition := config.ToCreateInstanceConfig()
	instanceDefinition.Name = instanceName

	// Call the Aura API to create the instance
	instance, err := deps.AClient.Instances.Create(instanceDefinition)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create instance: %v", err)), nil
	}

	// Format the response
	type createResult struct {
		Success       bool   `json:"success"`
		Message       string `json:"message"`
		ConfigUsed    string `json:"config_used"`
		Id            string `json:"id"`
		Name          string `json:"name"`
		Status        string `json:"status"`
		CloudProvider string `json:"cloud_provider"`
		Memory        string `json:"memory"`
		Type          string `json:"type"`
		URL           string `json:"url,omitempty"`
		Username      string `json:"username"`
		Password      string `json:"password"`
	}

	result := createResult{
		Success:       true,
		Message:       fmt.Sprintf("Instance created successfully from configuration '%s'", configLabel),
		ConfigUsed:    configLabel,
		Id:            instance.Data.Id,
		Name:          instance.Data.Name,
		CloudProvider: instance.Data.CloudProvider,
		Type:          instance.Data.Type,
		URL:           instance.Data.ConnectionUrl,
		Username:      instance.Data.Username,
		Password:      instance.Data.Password,
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// registerGetInstanceDetailsOutcome registers the get-instance-details outcome
func (r *OutcomeRegistry) registerGetInstanceDetailsOutcome() {
	r.Outcomes["get-instance-details"] = &Outcome{
		ID:          "get-instance-details",
		Name:        "Get Instance Details",
		Description: "Retrieve detailed information about a specific Neo4j Aura database instance. Returns comprehensive details including name, status, connection URL, cloud provider, region, memory, type, tenant ID, and importantly the Prometheus metrics endpoint URL for monitoring.",
		Type:        OutcomesTypeRead,
		ReadOnly:    true,
		Parameters: []OutcomeParameter{
			{
				Name:        "instance_id",
				Type:        "string",
				Description: "The ID of the instance to retrieve details for",
				Required:    true,
			},
		},
		Metadata: map[string]interface{}{
			"category": "instances",
		},
		Handler: executeGetInstanceDetails,
	}
}

// executeGetInstanceDetails implements the get-instance-details outcome
func executeGetInstanceDetails(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	if deps.AClient == nil {
		return mcp.NewToolResultError("Aura API Client is not initialized"), nil
	}

	// Validate and extract required parameter
	instanceID, ok := parameters["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("'instance_id' parameter is required and must be a non-empty string"), nil
	}

	// Get the instance details from Aura API
	instanceInfo, err := deps.AClient.Instances.Get(instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve instance details: %v. The instance may not exist or you may not have access to it.", err)), nil
	}

	// Format the response with all relevant details
	type instanceDetails aura.GetInstanceData

	details := instanceDetails{
		Id:            instanceInfo.Data.Id,
		Name:          instanceInfo.Data.Name,
		Status:        instanceInfo.Data.Status,
		ConnectionUrl: instanceInfo.Data.ConnectionUrl,
		CloudProvider: instanceInfo.Data.CloudProvider,
		Region:        instanceInfo.Data.Region,
		Memory:        instanceInfo.Data.Memory,
		Storage:       instanceInfo.Data.Storage,
		Type:          instanceInfo.Data.Type,
		TenantId:      instanceInfo.Data.TenantId,
		MetricsURL:    instanceInfo.Data.MetricsURL,
	}

	jsonData, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize instance details: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// registerDeleteInstanceOutcome registers the delete-instance outcome
func (r *OutcomeRegistry) registerDeleteInstanceOutcome() {
	r.Outcomes["delete-instance"] = &Outcome{
		ID:          "delete-instance",
		Name:        "Delete Instance",
		Description: "Permanently delete a Neo4j Aura database instance. This is a destructive operation that cannot be undone. Requires explicit confirmation via the 'confirm' parameter.",
		Type:        OutcomesTypeDelete,
		ReadOnly:    false,
		Parameters: []OutcomeParameter{
			{
				Name:        "instance_id",
				Type:        "string",
				Description: "The ID of the instance to delete",
				Required:    true,
			},
			{
				Name:        "confirm",
				Type:        "boolean",
				Description: "Must be set to true to confirm deletion. This is a safety measure to prevent accidental deletions.",
				Required:    true,
			},
		},
		Metadata: map[string]interface{}{
			"category":    "instances",
			"destructive": true,
			"warning":     "This operation permanently deletes the instance and all its data. This cannot be undone.",
		},
		Handler: executeDeleteInstance,
	}
}

// executeDeleteInstance implements the delete-instance outcome
func executeDeleteInstance(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	if deps.AClient == nil {
		return mcp.NewToolResultError("Aura API Client is not initialized"), nil
	}

	// Validate and extract required parameters
	instanceID, ok := parameters["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("'instance_id' parameter is required and must be a non-empty string"), nil
	}

	// Check for confirmation
	confirm, ok := parameters["confirm"].(bool)
	if !ok {
		return mcp.NewToolResultError("'confirm' parameter is required and must be a boolean (true to confirm deletion)"), nil
	}

	if !confirm {
		return mcp.NewToolResultError("Deletion not confirmed. Set 'confirm' to true to proceed with deletion. WARNING: This action cannot be undone."), nil
	}

	// Get instance details first to return information about what was deleted
	instanceInfo, err := deps.AClient.Instances.Get(instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve instance details before deletion: %v. The instance may not exist or you may not have access to it.", err)), nil
	}

	// Delete the instance using the Aura API client
	_, err = deps.AClient.Instances.Delete(instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete instance: %v", err)), nil
	}

	// Format the response
	type deleteResult struct {
		Success     bool   `json:"success"`
		Message     string `json:"message"`
		DeletedID   string `json:"deleted_id"`
		DeletedName string `json:"deleted_name"`
		Warning     string `json:"warning"`
	}

	result := deleteResult{
		Success:     true,
		Message:     fmt.Sprintf("Instance '%s' (ID: %s) has been successfully deleted", instanceInfo.Data.Name, instanceID),
		DeletedID:   instanceID,
		DeletedName: instanceInfo.Data.Name,
		Warning:     "This instance and all its data have been permanently deleted and cannot be recovered.",
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// registerCreateInstanceOutcome registers the create-instance outcome
func (r *OutcomeRegistry) registerCreateInstanceOutcome() {
	r.Outcomes["create-instance"] = &Outcome{
		ID:          "create-instance",
		Name:        "Create Instance",
		Description: "Create a new Neo4j Aura database instance with specified configuration. Returns the created instance details including ID, name, and connection information.",
		Type:        OutcomesTypeCreate,
		ReadOnly:    false,
		Parameters: []OutcomeParameter{
			{
				Name:        "name",
				Type:        "string",
				Description: "Name for the new instance",
				Required:    true,
			},
			{
				Name:        "cloud_provider",
				Type:        "string",
				Description: "Cloud provider: 'gcp', 'aws', or 'azure'",
				Required:    true,
			},
			{
				Name:        "region",
				Type:        "string",
				Description: "Cloud region (e.g., 'us-east-1' for AWS, 'us-central1' for GCP, 'eastus' for Azure)",
				Required:    true,
			},
			{
				Name:        "memory",
				Type:        "string",
				Description: "Memory size for the instance ('2GB', '4GB', '8GB', '16GB', '32GB', '48GB', '64GB', '96GB', '128GB', '192GB', '256GB', '384GB', '512GB', '768GB', '1024GB', '1536GB', '2048GB')",
				Required:    true,
			},
			{
				Name:        "type",
				Type:        "string",
				Description: "Instance type: 'free-db', 'professional-db', or 'business-critical','enterprise-db', 'enterprise-ds'",
				Required:    true,
			},
			{
				Name:        "tenantId",
				Type:        "string",
				Description: "The id of the project that the instance will be created in.",
				Required:    true,
			},
		},
		Metadata: map[string]interface{}{
			"category": "instances",
		},
		Handler: executeCreateInstance,
	}
}

// executeCreateInstance implements the create-instance outcome
func executeCreateInstance(ctx context.Context, parameters map[string]interface{}, deps *Dependencies) (*mcp.CallToolResult, error) {
	// These are supported parameters for creating an instance
	/*
		var supportedMemory = []string{
			"1GB", "2GB", "4GB", "8GB", "16GB", "24GB", "32GB", "48GB", "64GB", "128GB", "192GB", "256GB", "384GB", "512GB",
		}
		var supportedTypes = []string{
			"enterprise-db", "enterprise-ds", "professional-db", "professional-ds", "free-db", "business-critical",
		}
		var supportedCloudProviders = []string{"gcp", "aws", "azure"}
		var supportedVersions = []string{"5"}
		var supportedStorage = []string{
			"2GB", "4GB", "8GB", "16GB", "32GB", "48GB", "64GB", "96GB", "128GB", "192GB", "256GB", "384GB", "512GB",
			"768GB", "1024GB", "1536GB", "2048GB",
		}
	*/

	if deps.AClient == nil {
		return mcp.NewToolResultError("Aura API Client is not initialized"), nil
	}

	// Validate and extract required parameters
	name, ok := parameters["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("'name' parameter is required and must be a non-empty string"), nil
	}

	cloudProvider, ok := parameters["cloud_provider"].(string)
	if !ok || cloudProvider == "" {
		return mcp.NewToolResultError("'cloud_provider' parameter is required and must be one of: 'gcp', 'aws', 'azure'"), nil
	}

	// Validate cloud provider
	validProviders := map[string]bool{"gcp": true, "aws": true, "azure": true}
	if !validProviders[cloudProvider] {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid cloud_provider '%s'. Must be one of: 'gcp', 'aws', 'azure'", cloudProvider)), nil
	}

	region, ok := parameters["region"].(string)
	if !ok || region == "" {
		return mcp.NewToolResultError("'region' parameter is required and must be a non-empty string"), nil
	}

	memory, ok := parameters["memory"].(string)
	if !ok || memory == "" {
		return mcp.NewToolResultError("'memory' parameter is required (e.g., '2GB', '8GB', '16GB', '32GB', '64GB')"), nil
	}

	instanceType, ok := parameters["type"].(string)
	if !ok || instanceType == "" {
		return mcp.NewToolResultError("'type' parameter is required and must be one of: 'free', 'professional', 'enterprise'"), nil
	}

	// Validate instance type
	validTypes := map[string]bool{"enterprise-db": true, "enterprise-ds": true, "professional-db": true, "professional-ds": true, "free-db": true, "business-critical": true}
	if !validTypes[instanceType] {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid type '%s'. Must be one of: 'free', 'professional', 'enterprise'", instanceType)), nil
	}

	tenant, ok := parameters["tenantId"].(string)
	if !ok || instanceType == "" {
		return mcp.NewToolResultError("'tenantId' parameter is required"), nil
	}

	version := "5" // default

	// Create the instance using the Aura API client

	instanceDefinition := aura.CreateInstanceConfigData{
		Name:          name,
		CloudProvider: cloudProvider,
		Region:        region,
		Memory:        memory,
		Type:          instanceType,
		Version:       version,
		TenantId:      tenant,
	}

	// Call the Aura API to create the instance
	instance, err := deps.AClient.Instances.Create(&instanceDefinition)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create instance: %v", err)), nil
	}

	// Format the response
	type createResult struct {
		Success       bool   `json:"success"`
		Message       string `json:"message"`
		Id            string `json:"id"`
		Name          string `json:"name"`
		Status        string `json:"status"`
		CloudProvider string `json:"cloud_provider"`
		Memory        string `json:"memory"`
		Type          string `json:"type"`
		URL           string `json:"url,omitempty"`
		Username      string `json:"User"`
		Password      string `json:"Password"`
	}

	result := createResult{
		Success:       true,
		Message:       "Instance created successfully",
		Id:            instance.Data.Id,
		Name:          instance.Data.Name,
		CloudProvider: instance.Data.CloudProvider,
		Type:          instance.Data.Type,
		URL:           instance.Data.ConnectionUrl,
		Username:      instance.Data.Username,
		Password:      instance.Data.Password,
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
