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

