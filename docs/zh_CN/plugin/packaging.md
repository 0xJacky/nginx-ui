---
outline: [2, 3]
---

# 打包

插件以 gzip 压缩的 tar 归档（`.tar.gz`）分发，`plugin.json` 位于根目录。一个版本可以只发布一个**通用**插件包，也可以为**每个平台**各发布一个插件包，或者两者都发布。

## 构建插件包 {#building-a-package}

`nginx-ui plugin pack` 从插件目录构建插件包，传入密钥时还会签名：

```bash
nginx-ui plugin pack ./mydns io.github.example.mydns-1.0.0-linux-amd64.tar.gz --key mydns.key
```

任何能写 tar 归档的工具都可以，只要结果符合本页的要求。`nginx-ui plugin lint <插件包>` 会检查它。

## 结构 {#layout}

- `plugin.json` 位于归档的根目录，或者正好在一层目录之下（即 `tar czf x.tar.gz plugin-dir/` 生成的结构）。
- 根目录包含 `README.md` 和 `LICENSE`。它们是给安装插件的人和审核插件的人看的：缺少 `README.md` 是错误，缺少 `LICENSE` 是警告。每个版本的变更写在该版本的发布说明里，参见[版本](./catalog.md#releases)。
- 插件包包含 `server.executables` 声明的每个文件。
- 不包含符号链接或硬链接。

## 限制 {#limits}

| 限制 | 值 |
| --- | --- |
| 条目数（文件和目录） | 10,000 |
| 文件解压后的总大小 | 256 MiB |

插件包一旦超出限制，Nginx UI 会立即停止解压。为许多平台提供可执行文件的原生插件很快就会超出大小限制，请改为每个平台发布一个插件包。

## 安全路径 {#safe-paths}

每个归档条目和清单中的每个路径都必须是**安全的相对路径**：

- 用 `/` 分隔各段，绝不用 `\`；
- 不能为空，不能包含 NUL 字节；
- 不能是绝对路径，既不能是 `/foo`，也不能是 `C:foo` 这样的 Windows 驱动器路径；
- 已经是规范形式：没有 `.` 段，也没有 `//`；
- 不能是 `.` 或 `..`，也不能以 `../` 开头。

Nginx UI 会拒绝含有其他路径的插件包，而不会尝试修正它们，并确保解压出的任何内容都不会落到插件目录之外。

## 单一平台的插件包 {#per-platform-packages}

单一平台的插件包只包含一个平台的可执行文件。它的文件名以该平台结尾，`server.executables` 也只声明该平台：

```text
io.github.example.mydns-1.0.0-linux-arm64.tar.gz
```

```json [plugin.json]
"server": { "executables": { "linux-arm64": "dist/linux-arm64/mydns" } }
```

同一版本的所有插件包，除 `server.executables` 外清单都相同，除可执行文件和签名文件外文件也都相同，因此无论用户的平台拿到哪个插件包，他们批准的内容都是一样的。

通用插件包可以声明任意多个平台，并包含它声明的每个平台的可执行文件。没有 `server`，或使用解释型 `server.command` 的插件包可以在所有平台上运行。

文件名的构成参见[命名规则](./naming.md#package-file-names)。

## 可执行文件 {#executables}

解压后，Nginx UI 会为 `server.executables` 或 `server.command` 中的路径所指向的每个文件加上可执行权限（无论归档中如何记录），其他文件的权限位保持不变。插件包不提供的平台应从 `server.executables` 中省略，而不是声明了却缺少文件。

## 可复现的构建 {#reproducible-builds}

在工具链允许的范围内，请以确定的方式构建插件包：文件顺序稳定，除格式需要外不嵌入时间戳。这样插件包的校验和才有意义，任何人都能确认插件包是从公开的源码构建的。如果 `plugin.json` 是生成的，请按键排序写出。

## 安装 {#installing}

安装插件包时，Nginx UI 会：

1. 在上述限制内解压；
2. 校验清单，失败则丢弃全部内容；
3. 检查[签名](./signing.md)并得出信任等级；
4. 向用户显示插件的 ID、版本、权限和信任等级，并请求批准；
5. 安装它，如果之前有旧版本则替换。
