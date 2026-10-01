---
outline: [2, 3]
---

# 签名与信任

插件包自带签名，因此无论它来自插件目录、上传、离线插件包目录还是集群中的另一个节点，Nginx UI 都能知道是谁发布了它。签名者决定插件包的**信任等级**，而信任等级决定 Nginx UI 是否以及如何安装它。

签名使用 [minisign](https://jedisct1.github.io/minisign/)。

## 为插件包签名 {#signing-a-package}

生成一次密钥对，并妥善保管私钥：

```bash
minisign -G -p mydns.pub -s mydns.key
```

然后在插件包的其他所有文件都确定后，为每个插件包签名：

```bash
nginx-ui plugin pack ./mydns io.github.example.mydns-1.0.0.tar.gz --key mydns.key
# 或者直接为已有的插件包签名
nginx-ui plugin sign io.github.example.mydns-1.0.0.tar.gz --key mydns.key
```

签名会在插件包根目录添加两个文件：

| 文件 | 内容 |
| --- | --- |
| `plugin.sums` | 插件包其他所有文件的 SHA-256。 |
| `plugin.sums.minisig` | `plugin.sums` 的 minisign 签名。 |

之后对任何文件的修改都需要重新签名。同一版本的每个插件包都要单独签名。

### 手动签名 {#signing-by-hand}

`plugin.sums` 每个普通文件一行，格式与 `sha256sum` 的输出相同：

```text
<sha256>  <path>
```

- 64 个小写十六进制数字、两个空格，然后是相对于插件包根目录、以 `/` 分隔的路径；
- 每行以换行符结尾；不能有回车、空行、字节顺序标记或注释；
- 按路径的字节顺序排序（`LC_ALL=C sort`），每个路径只出现一次；
- 列出每个普通文件（包括空文件），但不包括根目录下的 `plugin.sums` 和 `plugin.sums.minisig`。不列出目录。

然后用 `minisign -S -m plugin.sums` 签名。两种 minisign 签名算法都可以接受。

## Nginx UI 如何检查插件包 {#how-nginx-ui-checks-a-package}

| 插件包包含 | 结果 |
| --- | --- |
| 没有签名，或只有两个文件中的一个 | 未签名 |
| Nginx UI 不认识的密钥所做的签名 | 未签名 |
| 无法解析的签名，或无法用它声明的密钥验证的签名 | **无效** |
| 有效的签名，但 `plugin.sums` 与文件不符 | **无效** |
| 有效的签名且文件相符 | 由该密钥签名 |

::: danger 警告
无效的插件包在签名之后被修改过。Nginx UI 总是拒绝它，即使在开发者模式下也是如此。`plugin.sums` 恰好列出插件包的所有普通文件及其正确的 SHA-256 时，才算相符。
:::

Nginx UI 会在安装前展示插件包时检查一次，在安装时再检查一次，对任何来源都是如此。插件目录中的校验和永远不能代替签名检查：它只能说明下载到的是插件目录所指的文件。

## 信任等级 {#trust-levels}

| 签名者 | 等级 |
| --- | --- |
| Nginx UI 项目的发布密钥 | `official` |
| 项目担保且未被吊销的合作伙伴密钥 | `verified` |
| 插件包所来自的目录条目的 `author_public_key`，或 Nginx UI **可信发布者**列表中的密钥 | `community` |
| 没有签名，或签名者未知 | `unsigned` |

等级排序为 `unsigned` < `community` < `verified` < `official`。插件目录和运维人员都不能把密钥提升到 `community` 以上。插件目录条目中的 `trust` 标签只用于显示，从不授予等级。

各等级允许的内容：

- **`official` 和 `verified`** 正常安装，并可以自动更新。
- **`community`** 只有在运维人员允许社区插件时才能安装，并且需要用户确认插件包是由作者而不是 Nginx UI 项目签名的。上传或复制到离线目录的插件包，只有当它的密钥在可信发布者列表中时才是 `community`。
- **`unsigned`** 只能在开发者模式下安装。

Nginx UI 会记录每个已安装插件的等级和签名者的密钥 ID，并显示出来。更新的插件包等级低于已安装插件时属于降级：自动更新会拒绝它，用户手动更新时会先收到警告。Nginx UI 自动安装的插件（例如 DNS-01 插件）必须是 `official`。

## 以社区作者身份发布 {#publishing-as-a-community-author}

大多数插件都是社区插件。发布步骤：

1. 用你的密钥为每个插件包签名。
2. 把你的公钥（`.pub` 文件中的 base64 那一行）作为 `author_public_key` 放进目录条目，参见[插件目录](./catalog.md#publisher)。从该条目下载并由该密钥签名的插件包就是 `community`。
3. 不通过插件目录安装你的插件包的人，需要把你的公钥添加到 **偏好设置 > 插件 > 可信发布者**。

## 合作伙伴插件 {#partner-plugins}

与 Nginx UI 项目合作的组织用自己的密钥签名，项目用发布密钥为这个密钥担保。它们的插件包在任何 Nginx UI 上都能得到 `verified`，即使该 Nginx UI 从未见过这个合作伙伴。

### 合作伙伴证书 {#the-partner-certificate}

合作伙伴的插件包带有其密钥的证书，即插件包根目录下的两个文件：

| 文件 | 内容 |
| --- | --- |
| `plugin.partner` | 合作伙伴的 minisign 公钥，格式与 `.pub` 文件相同。 |
| `plugin.partner.minisig` | 项目发布密钥对该文件的签名。 |

签名的可信注释写明合作伙伴的名称，还可以写明证书的最后有效日期：

```text
partner:example-corp
partner:example-corp;expires:2027-09-30
```

名称使用字母、数字、点和连字符。日期是 UTC 日历日期：证书在当天结束前一直有效。项目使用 `nginx-ui plugin certify` 签发证书，并为证书设置过期日期。

合作伙伴在为每个插件包签名之前，把这两个文件原样复制进去，使 `plugin.sums` 列出它们。一个证书适用于该密钥签名的所有插件包；续期的证书是修改过的文件，需要重新签名。

证书满足以下条件时才会被 Nginx UI 接受：`plugin.partner` 是有效的公钥，发布密钥验证了对它的签名，注释符合上述格式，证书未过期，且密钥未被吊销。不通过的证书不提供合作伙伴信任，但它本身永远不会使插件包无效：插件包会退回到其他信任来源。

### 合作伙伴密钥环 {#the-partner-keyring}

项目还在官方插件目录旁边发布一个签名的密钥环 `https://plugins.nginxui.com/v1/partners.json`，列出合作伙伴密钥和已吊销的密钥 ID。Nginx UI 刷新插件目录时会一起刷新它，把最近一份有效的副本保存在磁盘上，并且从不接受更旧的版本。密钥环中列出的密钥即使没有证书也能得到 `verified`；被它吊销的密钥无论插件包带有什么证书，都不会再得到 `verified`。吊销不会改变已安装插件的等级，但由被吊销密钥签名的更新会作为降级被拒绝。

## 开发者模式 {#developer-mode}

::: warning 注意
开发者模式位于 **偏好设置 > 插件**，默认关闭。开启后，Nginx UI 会从所有来源安装未签名的插件包，并把它们的发布者标记为未确认。它永远不会接受无效的插件包，不会放宽社区插件策略，也不会让未签名的插件参与自动更新。只在开发期间开启它。
:::
