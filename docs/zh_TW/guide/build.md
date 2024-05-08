# 構建

構建指南僅適用於開發人員或高階使用者。普通使用者應遵循 [快速入門](./getting-started) 指南。

## 依賴

- Make
- Golang 版本 1.22 或更高
- node.js 版本 21 或更高

你需要在構建專案之前執行以下命令更新瀏覽器列表資料庫。
  ```shell
  npx browserslist@latest --update-db
  ```

## 構建前端

請在 `app` 資料夾中執行以下命令。

```shell
pnpm install
pnpm build
```

## 構建後端

::: warning 警告
在構建後端之前應先構建前端，因為後端將嵌入前端構建的檔案。
:::

請在專案的根資料夾執行以下命令。

```shell
go build -tags=jsoniter -ldflags "$LD_FLAGS -X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
```
