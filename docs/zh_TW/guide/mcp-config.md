# MCP 配置文件管理

## 簡介

MCP 配置文件管理模組提供了一系列工具和資源，用於管理 Nginx 配置文件。這些功能允許 AI 代理和自動化工具執行各種配置文件操作，包括讀取、創建、修改和組織配置文件。

## 功能列表

### 獲取 Nginx 配置文件的根目錄路徑

- 類型：`tool`
- 名稱：`nginx_config_base_path`

### 列出配置文件

- 類型：`tool`
- 名稱：`nginx_config_list`

### 獲取配置文件內容

- 類型：`tool`
- 名稱：`nginx_config_get`

### 添加新的配置文件

- 類型：`tool`
- 名稱：`nginx_config_add`

### 修改現有配置文件

- 類型：`tool`
- 名稱：`nginx_config_modify`

### 重命名配置文件

- 類型：`tool`
- 名稱：`nginx_config_rename`

### 創建配置目錄

- 類型：`tool`
- 名稱：`nginx_config_mkdir`

### 歷史記錄

- 類型：`tool`
- 名稱：`nginx_config_history`

### 啟用配置文件

- 類型：`tool`
- 名稱：`nginx_config_enable`
- 描述：啟用之前創建的 Nginx 配置文件（在 sites-enabled 中創建符號連結）

## 使用示例

以下是一些使用 MCP 配置文件管理功能的示例：

### 獲取基礎路徑

```json
{
  "tool": "nginx_config_base_path",
  "parameters": {}
}
```

返回結果示例：

```json
{
  "base_path": "/etc/nginx"
}
```

### 列出配置文件

```json
{
  "tool": "nginx_config_list",
  "parameters": {
    "path": "/etc/nginx/conf.d"
  }
}
```

返回結果示例：

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

### 獲取配置文件內容

```json
{
  "tool": "nginx_config_get",
  "parameters": {
    "path": "/etc/nginx/conf.d/default.conf"
  }
}
```

### 修改配置文件

```json
{
  "tool": "nginx_config_modify",
  "parameters": {
    "path": "/etc/nginx/conf.d/default.conf",
    "content": "server {\n    listen 80;\n    server_name example.com;\n    location / {\n        root /usr/share/nginx/html;\n        index index.html;\n    }\n}"
  }
}
```

### 啟用配置文件

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

返回結果示例：

```json
{
  "status": "success",
  "message": "Site enabled and Nginx reloaded successfully",
  "source": "/etc/nginx/sites-available/my-site.conf",
  "destination": "/etc/nginx/sites-enabled/my-site.conf"
}
```

## 注意事項

- 所有路徑操作都是相對於 Nginx 配置基礎路徑的
- 配置文件修改會自動備份，可通過歷史記錄功能恢復
- 某些操作可能需要驗證配置文件語法正確性 