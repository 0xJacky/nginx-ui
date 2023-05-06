# Project Structure

## Root

```
.
├─ docs                    # documentations
├─ cmd                     # command-line tool
├─ frontend                # frontend build with vue 3
├─ server                  # backend build with golang
├─ resources               # additional resources, not for build
├─ template                # templates for nginx
├─ app.example.ini         # example configuration file
├─ main.go                 # entry point for server
└─ ...
```

## Documentations

```
.
├─ docs
│  ├─ .vitepress           # configurations directory
│  │  ├─ config
│  │  └─ theme
│  ├─ public               # resources
│  ├─ [language code]      # translations, e.g. zh_CN, zh_TW
│  ├─ guide
│  │  └─ *.md              # guide markdown files
│  └─ index.md             # index markdown
└─ ...
```

## Frontend

```
.
├─ frontend
│  ├─ public              # public resources
│  ├─ src                 # source code
│  │  ├─ api              # api to backend
│  │  ├─ assets           # public assets
│  │  ├─ components       # vue components
│  │  ├─ language         # translations, use vue3-gettext
│  │  ├─ layouts          # vue layouts
│  │  ├─ lib              # librarys, such as helper
│  │  ├─ pinia            # state management
│  │  ├─ routes           # vue routes
│  │  ├─ views            # vue views
│  │  ├─ gettext.ts       # define translations
│  │  ├─ style.less       # global style, using less syntax
│  │  ├─ dark.less        # dark style, using less syntax
│  │  └─ ...
│  └─ ...
└─ ...
```

## Backend

```
.
├─ server
│  ├─ internal             # internal packages
│  │  └─ ...
│  ├─ api                  # api to forntend
│  ├─ model                # model for generate
│  ├─ query                # generated request files by gen
│  ├─ router               # routers and middleware
│  ├─ service              # servie files
│  ├─ settings             # settings interface
│  ├─ test                 # unit test
│  └─ ...
├─ main.go                 # entry point for server
└─ ...
```

## Template


```
.
├─ template
│  ├─ block                # template for Nginx block
│  ├─ conf                 # template for Nginx configuration
│  └─ template.go          # embed template files to backend
└─ ...
```
