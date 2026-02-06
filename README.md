# MCP for Aura Infrastructure Management
A MCP Server that uses the Aura API for managing Aura Infrastructure. 

## Features

- Allows for Aura instance configurations to be defined in a JSON file which are then made available to LLM / Agent to use.  This simplifies usage as it removes the need for LLM / Agent to supply multiple configuration options.
- Retrieve a summary list of all Neo4j Aura database instances
- Get detailed info for a specific instance 
- Delete an instance
- Defaults to Read only.  This can be overriden with a configuration option. 


> **Note** It still remains possible to create instances by supplying all the needed parameters.  This is the fallback if a configuration file cannot be found. 

## Instance Configuration File

The server supports creating instances from pre-configured templates stored in a JSON file. This simplifies instance creation by avoiding the need to specify all parameters each time.

### Configuration File Setup

1. Copy the example configuration file:
   ```bash
   cp instance_configs.json.example instance_configs.json
   ```

2. Edit `instance_configs.json` and replace `YOUR_TENANT_ID_HERE` with your actual Neo4j Aura tenant/project ID

3. Customize the configurations as needed for your use case

### Configuration File Location

By default, the server looks for `instance_configs.json` in the current directory. You can specify a different location using the `INSTANCE_CONFIG_FILE` environment variable:

```bash
export INSTANCE_CONFIG_FILE=/path/to/your/configs.json
```

**Note:** The `instance_configs.json` file is in `.gitignore` to prevent accidentally committing sensitive tenant IDs. Always use the `.example` file as a template.

### Configuration File Format

The configuration file is a JSON file with the following structure:

```json
{
  "configurations": [
    {
      "label": "dev-small",
      "description": "Small development instance for testing",
      "name": "dev-instance",
      "cloud_provider": "gcp",
      "region": "us-central1",
      "memory": "2GB",
      "type": "professional-db",
      "tenant_id": "your-tenant-id-here",
      "version": "5"
    }
  ]
}
```

**Configuration Fields:**
- `label` (required): A friendly identifier for the configuration
- `description` (optional): Description of what this configuration is for
- `name` (required): Default name for instances created with this config
- `cloud_provider` (required): Cloud provider - 'gcp', 'aws', or 'azure'
- `region` (required): Cloud region (e.g., 'us-east-1', 'us-central1', 'eastus')
- `memory` (required): Memory size ('2GB', '4GB', '8GB', '16GB', etc.)
- `type` (required): Instance type - 'free-db', 'professional-db', 'business-critical', 'enterprise-db', 'enterprise-ds'
- `tenant_id` (required): The project ID where the instance will be created

### Using Configurations

1. **List available configurations:**
   Use the `list-instance-configs` outcome to see all available templates

2. **Create instance from configuration:**
   Use the `create-instance-from-config` outcome with a config label:
   ```json
   {
     "config_label": "dev-small",
     "override_name": "my-custom-name"  // optional
   }
   ```

### Benefits

- **Consistency:** Ensures all instances created with the same configuration are identical
- **Simplicity:** Only need to specify a label instead of 6+ parameters
- **Version Control:** Configuration file can be committed to version control
- **Easy Updates:** Centrally manage and update configurations
- **Better for LLMs/Agents:** They only need to remember labels instead of all parameters

## Prerequisites

- Go 1.25+ (see `go.mod`)
- Client Id and Secret for Neo4j Aura API
- Instance configuration file (see above section) 


## Installation
### Clone the repository 

```bash
git clone 
```

### Install Dependencies

```bash
cd  
go mod download

```


### Build

MCP for Aura API needs to be compiled before use.   Do this with

```Bash
go build -o ./bin/mcp-aura-infra-mgr ./cmd/mcp-aura-infra-mgr

```

### Testing

You can test before using with LLMs by using MCP Inspector. Set required environmental variables in MCP Inspector itself or beforehand.  If the latter, then you need to set CLIENT_ID and CLIENT_SECRET before running MCP Inspector. 

```bash
npx @modelcontextprotocol/inspector go run ./cmd/mcp-aura-infra-mgr

```


## Using with Claude Desktop

```json
{
  "mcpServers": {
    "mcp-aura-api": {
      "command": "<FULL PATH TO MCP BINARY>",
      "env": {
        "CLIENT_ID": "<YOUR AURA API CLIENT ID>",
        "CLIENT_SECRET": "<YOUR AURA API SECRET ID>",
        "READ_ONLY": "false",
        "INSTANCE_CONFIG_FILE": "<OPTIONAL: FULL PATH TO instance_configs.json>"
      }
    }
  }
}
```

**Note:** If `INSTANCE_CONFIG_FILE` is not specified, the server will look for `instance_configs.json` in the directory where the MCP server binary is executed.

