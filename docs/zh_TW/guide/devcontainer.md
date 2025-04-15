# 開發容器

如果您想參與本專案開發，需要設定開發環境。

## 必要條件

- Docker
- VSCode (Cursor)
- Git

## 設定步驟

1. 在 VSCode (Cursor) 中開啟指令面板
  - Mac: `Cmd`+`Shift`+`P`
  - Windows: `Ctrl`+`Shift`+`P`
2. 搜尋 `Dev Containers: 重新產生並重新開啟容器` 並點選
3. 等待容器啟動
4. 再次開啟指令面板
  - Mac: `Cmd`+`Shift`+`P`
  - Windows: `Ctrl`+`Shift`+`P`
5. 選擇 任務：執行任務 -> 啟動所有服務
6. 等待所有服務啟動完成

## 連接連接埠對映

| 連接連接埠 | 服務              |
|-------|-------------------|
| 3002  | 主應用程式            |
| 3003  | 文件              |
| 9000  | API 後端          |

## 服務清單

- nginx-ui
- nginx-ui-2
- casdoor
- chaltestsrv
- pebble

## 多節點開發

在主節點中新增以下環境設定：

```
name: nginx-ui-2
url: http://nginx-ui-2
token: nginx-ui-2
```
