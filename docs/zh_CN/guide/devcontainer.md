# 开发容器

如果您想参与本项目开发，需要设置开发环境。

## 前提条件

- Docker
- VSCode (Cursor)
- Git

## 设置步骤

1. 在 VSCode (Cursor) 中打开命令面板
  - Mac: `Cmd`+`Shift`+`P`
  - Windows: `Ctrl`+`Shift`+`P`
2. 搜索 `Dev Containers: 重新生成并重新打开容器` 并点击
3. 等待容器启动
4. 再次打开命令面板
  - Mac: `Cmd`+`Shift`+`P`
  - Windows: `Ctrl`+`Shift`+`P`
5. 选择 任务: 运行任务 -> 启动所有服务
6. 等待所有服务启动完成

## 端口映射

| 端口   | 服务              |
|-------|-------------------|
| 3002  | 主应用            |
| 3003  | 文档              |
| 9000  | API 后端          |

## 服务列表

- nginx-ui
- nginx-ui-2
- casdoor
- chaltestsrv
- pebble

## 多节点开发

在主节点中添加以下环境配置：

```
name: nginx-ui-2
url: http://nginx-ui-2
token: nginx-ui-2
```
