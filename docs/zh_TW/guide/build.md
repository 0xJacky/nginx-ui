# 建構

建構指南僅適用於開發人員或高階使用者。普通使用者應遵循 [快速入門](./getting-started) 指南。

## 依賴

- Make
- Golang 版本 1.23 或更高
- node.js 版本 21 或更高

你需要在建構專案之前執行以下命令更新瀏覽器列表資料庫。
  ```shell
  npx browserslist@latest --update-db
  ```

## 建構前端

請在 `app` 資料夾中執行以下命令。

```shell
pnpm install
pnpm build
```

## 建構後端

::: warning 警告
在建構後端之前應先建構前端，因為後端將嵌入前端建構的檔案。
:::

請在專案的根資料夾執行以下命令。

```shell
go generate
go build -tags=jsoniter -ldflags "$LD_FLAGS -X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
```
