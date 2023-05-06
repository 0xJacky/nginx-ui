# 项目结构

## 根目录

```
.
├─ docs                    # 文档目录
├─ cmd                     # 命令行工具
├─ frontend                # 使用 Vue 3 构建的前端
├─ server                  # 使用 Golang 构建的后端
├─ resources               # 其他资源，不参与构建
├─ template                # 用于 Nginx 的模板文件
├─ app.example.ini         # 配置文件的示例
├─ main.go                 # 服务器入口
└─ ...
```

## 文档目录

```
.
├─ docs
│  ├─ .vitepress           # 配置目录
│  │  ├─ config
│  │  └─ theme
│  ├─ public               # 资源
│  ├─ [language code]      # 翻译，文件夹名为语言代码，例如 zh_CN, zh_TW
│  ├─ guide
│  │  └─ *.md              # 手册 markdown 文件
│  └─ index.md             # 首页 markdown 文件
└─ ...
```

## 前端

```
.
├─ frontend
│  ├─ public              # 公共资源
│  ├─ src                 # 源代码
│  │  ├─ api              # 向后端发起请求的 API
│  │  ├─ assets           # 公共资源
│  │  ├─ components       # Vue 组件
│  │  ├─ language         # 翻译，使用 vue3-gettext
│  │  ├─ layouts          # Vue 布局
│  │  ├─ lib              # 库文件，如帮助函数
│  │  ├─ pinia            # 状态管理
│  │  ├─ routes           # Vue 路由
│  │  ├─ views            # Vue 视图
│  │  ├─ gettext.ts       # 定义翻译
│  │  ├─ style.less       # 全局样式，使用 less 语法
│  │  ├─ dark.less        # 暗黑主题样式，使用 less 语法
│  │  └─ ...
│  └─ ...
└─ ...
```

## 后端

```
.
├─ server
│  ├─ internal             # 内部包
│  │  └─ ...
│  ├─ api                  # 向前端提供的 API
│  ├─ model                # 自动生成的模型
│  ├─ query                # gen 自动生成的请求文件
│  ├─ router               # 路由和中间件
│  ├─ service              # 服务文件
│  ├─ settings             # 配置接口
│  ├─ test                 # 单元测试
│  └─ ...
├─ main.go                 # 后端入口
└─ ...
```

## 模板

```
.
├─ template
│  ├─ block                # Nginx 块配置模板
│  ├─ conf                 # Nginx 配置模板
│  └─ template.go          # 嵌入模板文件至后端
└─ ...
```
