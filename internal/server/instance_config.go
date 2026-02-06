package server

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/LackOfMorals/aura-client"
)

// InstanceConfig represents a stored configuration for creating an instance
type InstanceConfig struct {
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

// InstanceConfigFile represents the JSON file containing multiple instance configurations
type InstanceConfigFile struct {
	Configurations []InstanceConfig `json:"configurations"`
}

// LoadInstanceConfigurations loads instance configurations from a JSON file
// The file path can be specified via environment variable INSTANCE_CONFIG_FILE
// or defaults to "./instance_configs.json"
func LoadInstanceConfigurations(configPath string) (*InstanceConfigFile, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("instance configuration file not found at '%s'. Create this file with your instance configurations or set INSTANCE_CONFIG_FILE environment variable", configPath)
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read instance configuration file: %w", err)
	}

	// Parse JSON
	var configFile InstanceConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("failed to parse instance configuration file: %w", err)
	}

	// Validate configurations
	if len(configFile.Configurations) == 0 {
		return nil, fmt.Errorf("no configurations found in file")
	}

	return &configFile, nil
}

// GetConfigByLabel retrieves a configuration by its label
func (cf *InstanceConfigFile) GetConfigByLabel(label string) (*InstanceConfig, error) {
	for _, config := range cf.Configurations {
		if config.Label == label {
			return &config, nil
		}
	}
	return nil, fmt.Errorf("configuration with label '%s' not found", label)
}

// ValidateConfig validates an instance configuration
func (ic *InstanceConfig) ValidateConfig() error {
	if ic.Label == "" {
		return fmt.Errorf("label is required")
	}
	if ic.Name == "" {
		return fmt.Errorf("name is required for configuration '%s'", ic.Label)
	}
	if ic.CloudProvider == "" {
		return fmt.Errorf("cloud_provider is required for configuration '%s'", ic.Label)
	}
	if ic.Region == "" {
		return fmt.Errorf("region is required for configuration '%s'", ic.Label)
	}
	if ic.Memory == "" {
		return fmt.Errorf("memory is required for configuration '%s'", ic.Label)
	}
	if ic.Type == "" {
		return fmt.Errorf("type is required for configuration '%s'", ic.Label)
	}
	if ic.TenantId == "" {
		return fmt.Errorf("tenant_id is required for configuration '%s'", ic.Label)
	}

	// Validate cloud provider
	validProviders := map[string]bool{"gcp": true, "aws": true, "azure": true}
	if !validProviders[ic.CloudProvider] {
		return fmt.Errorf("invalid cloud_provider '%s' in configuration '%s'. Must be one of: 'gcp', 'aws', 'azure'", ic.CloudProvider, ic.Label)
	}

	// Validate instance type
	validTypes := map[string]bool{
		"enterprise-db": true, "enterprise-ds": true,
		"professional-db": true, "professional-ds": true,
		"free-db": true, "business-critical": true,
	}
	if !validTypes[ic.Type] {
		return fmt.Errorf("invalid type '%s' in configuration '%s'. Must be one of: 'enterprise-db', 'enterprise-ds', 'professional-db', 'professional-ds', 'free-db', 'business-critical'", ic.Type, ic.Label)
	}

	return nil
}

// ToCreateInstanceConfig converts an InstanceConfig to the Aura API CreateInstanceConfigData
func (ic *InstanceConfig) ToCreateInstanceConfig() *aura.CreateInstanceConfigData {
	version := ic.Version
	if version == "" {
		version = "5" // default
	}

	return &aura.CreateInstanceConfigData{
		Name:          ic.Name,
		CloudProvider: ic.CloudProvider,
		Region:        ic.Region,
		Memory:        ic.Memory,
		Type:          ic.Type,
		Version:       version,
		TenantId:      ic.TenantId,
	}
}
