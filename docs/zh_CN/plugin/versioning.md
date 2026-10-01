---
outline: [2, 3]
---

# 版本与兼容性

插件中有三个版本号，它们的含义各不相同：

| 版本 | 位置 | 表示什么 |
| --- | --- | --- |
| `api_version` | `plugin.json` 和握手 | 插件协议的代次，目前为 `1`。 |
| `version` | `plugin.json` | 插件自身的发布版本，使用语义化版本。 |
| `min_nginx_ui_version` | `plugin.json` | 插件支持的最低 Nginx UI 版本。 |

## 协议代次 {#the-protocol-generation}

只有协议发生不兼容的变更时，`api_version` 才会改变：例如删除字段、重命名键、错误码的含义改变。在同一代次内，协议只会增加内容：新的可选字段、新的能力、新的事件、新的宿主方法、新的插槽。以下两条规则保证了双向兼容：

- **忽略不认识的内容。** 插件忽略它不认识的 JSON 成员、事件类型和插槽名称，因此较新的 Nginx UI 不会破坏它。
- **不要依赖可选内容。** 插件把每个可选字段都视为可能缺失，因此不发送该字段的较旧 Nginx UI 也能正常工作。

插件的 `api_version` 必须等于 Nginx UI 实现的代次。两者之间没有协商：不一致时握手会失败，并给出明确的错误。当第二代出现时，两代会并列提供文档，直到第一代在提前通知后退役。

## 插件版本 {#the-plugin-version}

`version` 由你决定。请使用语义化版本：需要用户采取行动的变更提升主版本号，新功能提升次版本号，修复提升修订号。`1.2.0-beta.1` 这样的预发布版本会让该版本进入较不稳定的[发布渠道](./catalog.md#release-channels)。

## 最低 Nginx UI 版本 {#the-minimum-nginx-ui-version}

`min_nginx_ui_version` 指定插件支持的最低 Nginx UI 版本，例如因为插件使用了该版本新增的宿主方法或插槽：

```json [plugin.json]
"min_nginx_ui_version": "2.7.0"
```

它仅作建议。Nginx UI 会显示它，也可能拒绝启用需要更新版本的插件，但它不属于协议，插件仍应检查自己使用的可选功能是否存在。在浏览器包中，请检测运行时的可选成员（例如 `registry.loadChunk`）是否存在，而不是依赖版本号。
