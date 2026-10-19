# 构建

构建指南仅适用于开发人员或高级用户。普通用户应遵循 [快速入门](./getting-started) 指南。

## 依赖

- Make
- Go 版本 1.27.2 或更高（与 `go.mod` 要求一致）
- Bun 版本 1.4.3（由 `package.json` 固定）
- Node.js 版本 21 或更高（用于下方的 `npx browserslist` 命令）

你需要在构建项目之前执行以下命令更新浏览器列表数据库。
  ```shell
  npx browserslist@latest --update-db
  ```

## 构建前端

请在 `app` 目录中执行以下命令。

```shell
bun install
bun run build
```

## 构建后端

::: warning 警告
在构建后端之前应先构建前端，因为后端将嵌入前端构建的文件。
:::

请在项目的根目录执行以下命令。

```shell
go generate
go build -tags=jsoniter -ldflags "$LD_FLAGS -X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
```
