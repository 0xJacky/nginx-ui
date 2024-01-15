# 專案結構

## 根目錄

```
.
├─ docs                    # 手冊資料夾
├─ cmd                     # 命令列工具
├─ app                     # 使用 Vue 3 構建的前端
├─ resources               # 其他資源，不參與構建
├─ template                # 用於 Nginx 的模板檔案
├─ app.example.ini         # 配置檔案的示例
├─ main.go                 # 伺服器入口
└─ ...
```

## 手冊資料夾

```
.
├─ docs
│  ├─ .vitepress           # 配置資料夾
│  │  ├─ config
│  │  └─ theme
│  ├─ public               # 資源
│  ├─ [language code]      # 翻譯，資料夾名為語言程式碼，例如 zh_CN, zh_TW
│  ├─ guide
│  │  └─ *.md              # 手冊 markdown 檔案
│  └─ index.md             # 首頁 markdown 檔案
└─ ...
```

## 前端

```
.
├─ app
│  ├─ public              # 公共資源
│  ├─ src                 # 原始碼
│  │  ├─ api              # 向後端發起請求的 API
│  │  ├─ assets           # 公共資源
│  │  ├─ components       # Vue 元件
│  │  ├─ language         # 翻譯，使用 vue3-gettext
│  │  ├─ layouts          # Vue 佈局
│  │  ├─ lib              # 庫檔案，如幫助函式
│  │  ├─ pinia            # 狀態管理
│  │  ├─ routes           # Vue 路由
│  │  ├─ views            # Vue 檢視
│  │  ├─ gettext.ts       # 定義翻譯
│  │  ├─ style.css        # 集成 tailwind
│  │  └─ ...
│  └─ ...
└─ ...
```

## 後端

```
.
├─ internal             # 內部包
├─ api                  # 向前端提供的 API
├─ model                # 數據庫模型
├─ query                # gen 自動生成的數據庫查詢文件
├─ router               # 路由和中間件
├─ settings             # 後端配置
├─ test                 # 單元測試
├─ main.go              # 主程序入口
└─ ...
```

## 模板

```
.
├─ template
│  ├─ block                # Nginx 塊配置模板
│  ├─ conf                 # Nginx 配置模板
│  └─ template.go          # 嵌入模板檔案至後端
└─ ...
```
