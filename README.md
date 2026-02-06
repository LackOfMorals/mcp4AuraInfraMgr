# MCP for Aura Infrastructure Management
A MCP Server that uses the Aura API for managing Aura Infrastructure. 

## Features

- Allows for Aura instance configurations to be defined in a JSON file which are then made available to LLM / Agent to use.  This simplifies usage as it removes the need for LLM / Agent to supply multiple configuration options.
- Retrieve a summary list of all Neo4j Aura database instances
- Get detailed info for a specific instance 
- Delete an instance
- Defaults to Read only.  This can be overriden with a configuration option. 

## Prerequisites

- Go 1.25+ (see `go.mod`)
- Client Id and Secret for Neo4j Aura API 


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
npx @modelcontextprotocol/inspector go run ./cmd/mcp-aura-api

```


## Using with Claude Desktop

```Text
{
  "mcpServers": {
    "mcp-aura-api": {
      "command": "<FULL PATH TO MCP BINARY>",
      "env": {
        "CLIENT_ID":"<YOUR AURA API CLIENT ID>",
        "CLIENT_SECRET":"<YOUR AURA API SECRET ID>"
        "READ_ONLY": <Set to False to allow for changes. Default value is True if not set>
      }
    }
  }
}
```

