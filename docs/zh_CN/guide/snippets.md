---
outline: [2, 3]
---

# 片段

片段把一段 Nginx 配置放在一处统一管理，例如缓存规则、安全响应头或网站的 PHP 处理配置。网站通过 include 引用片段，因此修改片段会同时改变所有引用它的网站。片段位于 **网站管理 > 片段**。

## 片段的存放位置 {#where-snippets-live}

每个片段都是 Nginx 配置目录下 `snippets` 目录中的一个文件，例如 `/etc/nginx/snippets/static-cache.conf`。许多发行版本来就把片段放在这里，因此 Nginx 自带的片段（例如 Debian 的 `snippets/fastcgi-php.conf`）也会列出来。

文件名以 `.conf` 结尾、且只包含字母、数字、点、连字符和下划线的文件才算片段。`snippets/plugins` 目录为保留目录，不会被处理。

## 使用片段 {#using-a-snippet}

在网站中使用片段有两种方式：

- **引用。** 把片段页面显示的指令加到 `server` 或 `location` 块中：

  ```nginx
  location /assets/ {
      include snippets/static-cache.conf;
  }
  ```

  之后片段的每次修改都会作用到这个网站。
- **插入。** 在网站编辑器中打开配置模板面板，选择片段并点击 **插入**。片段的内容会被复制到网站中，之后的修改不会作用到它。同一对话框中的 **引用** 会替你添加 include 指令。

保存片段时会用 `nginx -t` 测试整体配置并重载 Nginx。Nginx 拒绝配置时，会保留原来的片段并显示错误。

::: warning 注意
仍被网站引用的片段无法删除，否则 Nginx 会拒绝配置。片段页面会列出引用它的文件。
:::

## 名称、描述与变量 {#name-description-and-variables}

片段页面显示的名称和描述保存在文件顶部的文件头中。文件头的每一行都是注释，因此 Nginx 始终可以引用这个文件：

```nginx [snippets/static-cache.conf]
# Nginx UI Template Start
# name = "Static file cache"
#
# [description]
# en = "Cache images, scripts and styles for a week"
# Nginx UI Template End

expires 7d;
add_header Cache-Control "public";
```

文件头使用[配置模板](./nginx-ui-template.md)的格式，只是写成了注释。片段也可以用同样的方式在其中声明变量：

```nginx [snippets/hsts.conf]
# Nginx UI Template Start
# name = "HSTS"
#
# [variables.maxAge]
# type = "string"
# name = { en = "Max Age" }
# value = "31536000"
# Nginx UI Template End

add_header Strict-Transport-Security "max-age={{ .maxAge }}" always;
```

带变量的片段由配置模板面板填写变量，只能插入使用。Nginx 无法直接引用它，因为 `{{ }}` 占位符不是 Nginx 配置。变量需要直接编辑文件头来添加（例如在 **配置管理** 中），片段页面会保留它们。

## 同步到节点 {#synchronizing-to-nodes}

片段页面的 **同步** 会把所有片段复制到你选择的节点并保持更新：保存的片段会复制过去，删除的片段也会从节点上删除。你可以选择是否替换节点上已存在的同名片段。

网站同步到某个节点时，会带上它引用的片段。这些片段只会在节点上缺少时才创建，绝不会替换节点上已有的副本，因为发行版自带的片段在不同节点上可能本来就应该不同。如需替换，请同步片段本身。

::: tip 提示
片段同步使用的是[使用集群节点管理多主机 Nginx](./manage-multi-host-nginx-with-cluster.md) 中的目录部署，因此在 **配置管理** 中，snippets 目录也会显示相同的同步目标。
:::
