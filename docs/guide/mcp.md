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

Create a service token in **Preferences > Access Tokens** and grant it the
smallest MCP scope the client needs:

- `mcp:read` permits resources and read-only tools.
- `mcp:write` permits mutating tools and includes MCP read access.

Send the token in the `Authorization` header:

```http
Authorization: Bearer nui_pat_...
```

Tokens are displayed only once. Store them in a secret manager and set an
expiration date whenever possible. Credentials in URL query parameters are
rejected to prevent leakage through logs and browser history.

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
