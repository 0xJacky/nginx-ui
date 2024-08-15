# Build

The build guide is intended for developers or advanced users only.
Regular users should follow the [quick start](./getting-started) guide.

## Prerequisites

- Make.
- Golang version 1.23 or higher.
- node.js version 21 or higher.

You should execute the following command to update browser list database before build project.
  ```shell
  npx browserslist@latest --update-db
  ```

## Build Frontend

Please execute the following command in `app` directory.

```shell
pnpm install
pnpm build
```

## Build Backend

::: warning
Before building the backend, the app should be built first because the backend will embed the app distribution.
:::

Please execute the following command in the project root directory.

```shell
go build -tags=jsoniter -ldflags "$LD_FLAGS -X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
```
