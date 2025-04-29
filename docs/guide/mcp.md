# MCP Module

## Introduction

MCP (Model Context Protocol) is a special interface provided by Nginx UI that allows AI agents to interact with Nginx UI. Through MCP, AI models can access and manage Nginx configuration files, perform Nginx-related operations (such as restart, reload), and get Nginx running status.

## Feature Overview

The MCP module is divided into two main functional areas:

- [Configuration File Management](./mcp-config.md) - Various operations for managing Nginx configuration files
- [Nginx Service Management](./mcp-nginx.md) - Control and monitor Nginx service status

## Interface

The MCP interface is accessible through the `/mcp` path and provides streaming via SSE.

## Authentication

The MCP interface is authenticated using the `node_secret` query parameter.

For example:

```
http://localhost:9000/mcp?node_secret=<your_node_secret>
```

### Resources

Resources are readable information provided by MCP, such as Nginx status.

### Tools

Tools are executable operations provided by MCP, such as restarting Nginx, modifying configuration files, etc.

## Use Cases

MCP is mainly used in the following scenarios:

1. AI-driven Nginx configuration management
2. Integration with automated operations tools
3. Integration of third-party systems with Nginx UI
4. Providing machine-readable APIs for automation scripts 