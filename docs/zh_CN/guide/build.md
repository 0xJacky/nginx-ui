# 构建

构建指南仅适用于开发人员或高级用户。普通用户应遵循 [快速入门](./getting-started) 指南。

## 依赖

- Make
- Golang 版本 1.21 或更高
- node.js 版本 21 或更高

你需要在构建项目之前执行以下命令更新浏览器列表数据库。
  ```shell
  npx browserslist@latest --update-db
  ```

## 构建前端

请在 `app` 目录中执行以下命令。

```shell
pnpm install
pnpm build
```

## 构建后端

::: warning 警告
在构建后端之前应先构建前端，因为后端将嵌入前端构建的文件。
:::

请在项目的根目录执行以下命令。

```shell
go build -o nginx-ui -v main.go
```
