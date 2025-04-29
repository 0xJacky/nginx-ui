# MCP 配置文件管理

## 简介

MCP 配置文件管理模块提供了一系列工具和资源，用于管理 Nginx 配置文件。这些功能允许 AI 代理和自动化工具执行各种配置文件操作，包括读取、创建、修改和组织配置文件。

## 功能列表

### 获取 Nginx 配置文件的根目录路径

- 类型：`tool`
- 名称：`nginx_config_base_path`
- 描述：获取 Nginx 配置文件的根目录路径

### 列出配置文件

- 类型：`tool`
- 名称：`nginx_config_list`
- 描述：获取指定目录下的配置文件和子目录列表

### 获取配置文件内容

- 类型：`tool`
- 名称：`nginx_config_get`
- 描述：读取指定配置文件的内容

### 添加新的配置文件

- 类型：`tool`
- 名称：`nginx_config_add`
- 描述：创建新的配置文件

### 修改现有配置文件

- 类型：`tool`
- 名称：`nginx_config_modify`
- 描述：更新现有配置文件的内容

### 重命名配置文件

- 类型：`tool`
- 名称：`nginx_config_rename`
- 描述：修改配置文件的名称或路径

### 创建配置目录

- 类型：`tool`
- 名称：`nginx_config_mkdir`
- 描述：创建新的配置目录

### 历史记录

- 类型：`tool`
- 名称：`nginx_config_history`
- 描述：获取配置文件的修改历史记录
