# Devcontainer

You'll need to set up a development environment if you want to develop on this project.

## Prerequisites

- Docker
- VSCode (Cursor)
- Git

## Setup

1. Open the Command Palette in VSCode (Cursor)
  - Mac: `Cmd`+`Shift`+`P`
  - Windows: `Ctrl`+`Shift`+`P`
2. Search for `Dev Containers: Rebuild and Reopen in Container` and click on it
3. Wait for the container to start
4. Open the Command Palette in VSCode (Cursor)
  - Mac: `Cmd`+`Shift`+`P`
  - Windows: `Ctrl`+`Shift`+`P`
5. Select Tasks: Run Task -> Start all services
6. Wait for the services to start

## Optional documentation search in Cursor

When researching Nginx directives or troubleshooting a configuration, you can
add [Parallel Search MCP](https://docs.parallel.ai/integrations/mcp/search-mcp)
to Cursor's development tools. It provides `web_search` for finding sources and
`web_fetch` for reading a specific page. Anonymous search uses Fast mode and
requires no Parallel account or API key. Free usage is rate limited.

1. Install Node.js 22.12 or newer, with `npx` available on Cursor's PATH. If
   Cursor runs MCP tools inside the devcontainer, install Node.js there.
2. Open `.cursor/parallel-search.example.json` in the repository and merge its
   `parallel-search` entry into the `mcpServers` object in `.cursor/mcp.json`.
   Keep the existing `eslint`, `context7`, and any other servers. The example
   pins the `mcp-remote` dependency, which `npx` downloads on first use, and
   bridges local stdio to the remote HTTPS MCP endpoint.
3. Reload Cursor's MCP configuration and enable `parallel-search` in its MCP
   settings. Confirm that `web_search` and `web_fetch` appear in the tool list.
4. Ask Cursor to use `web_search` to find the official Nginx documentation for
   `proxy_read_timeout`, then use `web_fetch` on
   `https://nginx.org/en/docs/http/ngx_http_proxy_module.html` to read the exact
   directive description. Check the source before applying a configuration.

Search queries and requested URLs are sent to Parallel. Use public documentation
queries and omit credentials, private hostnames, and confidential configuration
content. This setup adds tools to your development editor; it does not enable
web search in Nginx UI's built-in ChatGPT assistant. To disable it, remove only
the `parallel-search` entry from your local MCP configuration.

## Ports

| Port  | Service          |
|-------|------------------|
| 3002  | App              |
| 3003  | Documentation    |
| 9000  | API Backend      |


## Services

- nginx-ui
- nginx-ui-2
- casdoor
- chaltestsrv
- pebble

## Multi-node development

Add the following enviroment in the main node:

```
name: nginx-ui-2
url: http://nginx-ui-2
token: nginx-ui-2
```

