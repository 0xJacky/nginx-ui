# MCP Configuration File Management

## Introduction

The MCP Configuration File Management module provides a set of tools and resources for managing Nginx configuration files. These features allow AI agents and automation tools to perform various configuration file operations, including reading, creating, modifying, and organizing configuration files.

## Feature List

### Get Nginx Configuration File Base Path

- Type: `tool`
- Name: `nginx_config_base_path`

### List Configuration Files

- Type: `tool`
- Name: `nginx_config_list`

### Get Configuration File Content

- Type: `tool`
- Name: `nginx_config_get`

### Add New Configuration File

- Type: `tool`
- Name: `nginx_config_add`

### Modify Existing Configuration File

- Type: `tool`
- Name: `nginx_config_modify`

### Rename Configuration File

- Type: `tool`
- Name: `nginx_config_rename`

### Create Configuration Directory

- Type: `tool`
- Name: `nginx_config_mkdir`

### History

- Type: `tool`
- Name: `nginx_config_history`

### Enable Configuration File

- Type: `tool`
- Name: `nginx_config_enable`
- Description: Enable a previously created Nginx configuration (creates symlink in sites-enabled)

## Usage Examples

Here are some examples of using MCP Configuration File Management features:

### Get Base Path

```json
{
  "tool": "nginx_config_base_path",
  "parameters": {}
}
```

Example response:

```json
{
  "base_path": "/etc/nginx"
}
```

### List Configuration Files

```json
{
  "tool": "nginx_config_list",
  "parameters": {
    "path": "/etc/nginx/conf.d"
  }
}
```

Example response:

```json
{
  "files": [
    {
      "name": "default.conf",
      "is_dir": false,
      "path": "/etc/nginx/conf.d/default.conf"
    },
    {
      "name": "example.conf",
      "is_dir": false,
      "path": "/etc/nginx/conf.d/example.conf"
    }
  ]
}
```

### Get Configuration File Content

```json
{
  "tool": "nginx_config_get",
  "parameters": {
    "path": "/etc/nginx/conf.d/default.conf"
  }
}
```

### Modify Configuration File

```json
{
  "tool": "nginx_config_modify",
  "parameters": {
    "path": "/etc/nginx/conf.d/default.conf",
    "content": "server {\n    listen 80;\n    server_name example.com;\n    location / {\n        root /usr/share/nginx/html;\n        index index.html;\n    }\n}"
  }
}
```

### Enable Configuration File

```json
{
  "tool": "nginx_config_enable",
  "parameters": {
    "name": "my-site.conf",
    "base_dir": "sites-available",
    "overwrite": false
  }
}
```

Example response:

```json
{
  "status": "success",
  "message": "Site enabled and Nginx reloaded successfully",
  "source": "/etc/nginx/sites-available/my-site.conf",
  "destination": "/etc/nginx/sites-enabled/my-site.conf"
}
```

## Important Notes

- All path operations are relative to the Nginx configuration base path
- Configuration file modifications are automatically backed up and can be restored using the history feature
- Some operations may require validation of configuration file syntax 