# Nginx Log

Nginx UI 会列出从 Nginx 配置中发现的日志文件，以及 Nginx 默认的访问日志和错误日志。你可以分页查看日志，也可以实时跟踪。这些功能无需任何设置。

## 日志分析

日志分析内置在 Nginx UI 中。开启索引后，Nginx UI 会为列出的访问日志和错误日志建立索引，并提供：

- 结构化搜索，支持过滤条件、时间范围和分页结果，访问日志和错误日志均可使用；
- 流量面板，包括请求数、访客数、状态码、热门页面、浏览器、操作系统和设备；
- 按国家展示的访客地图；安装 GeoLite2 City 数据库后，还可以细分到省份和城市。

在日志列表中点击 **启用高级索引**，或设置下方的 [IndexingEnabled](#indexingenabled) 即可开启索引。Nginx UI 会先为已有日志建立索引，之后只读取新追加的内容。轮转后的文件，包括被 logrotate 重命名、被 `copytruncate` 复制以及被压缩为 `.gz` 的文件，会按内容识别，不会重复索引。

索引是一个列式存储，保存在 [IndexPath](#indexpath) 下的 `logstore` 目录中，体积通常远小于原始日志。

::: tip 从 v2 升级
v2 使用 Bleve 索引。v3 首次启动时会删除旧的索引目录，并把所有日志重新索引到新的存储中。无需任何操作，但日志较大时重建需要一些时间。
:::

### 搜索语法

搜索框中的关键词和 `字段:值` 过滤条件之间是「与」的关系。在条件前加 `-` 表示排除，用双引号可以把一个短语作为整体。普通关键词按不区分大小写的子串匹配原始日志行。

| 过滤条件 | 示例 |
|----------|------|
| 状态码、状态码类别或范围 | `status:404`、`status:5xx`、`status:400-499`、`status:>=500` |
| 请求方法 | `method:POST` |
| 客户端地址或网段 | `ip:10.0.0.0/8`、`ip:2001:db8::/32` |
| 路径、User Agent、Referer（子串） | `path:/api/`、`ua:curl`、`referer:google` |
| 浏览器、操作系统、设备、省份、城市（子串） | `browser:firefox`、`os:android`、`city:beijing` |
| 国家代码 | `country:CN` |
| 发送字节数 | `bytes:>1000` |
| 请求耗时（秒） | `rt:>0.5`、`rt:0.1..0.5` |
| 错误日志级别 | `level:error`、`-level:warn` |

看起来像过滤条件但无法识别的内容会按普通文本搜索，日志页面会对此给出提示。

### IP 地理位置

国家信息来自 Nginx UI 内置的 GeoLite2 Country 数据库，因此无需下载任何文件即可显示世界地图。省份和城市需要 GeoLite2 City 数据库，可以在 **偏好设置 > GeoLite** 中下载，文件保存为 `app.ini` 同一目录下的 `GeoLite2-City.mmdb`。也可以使用自定义数据库，参见 [IndexCustomMMDB](#indexcustommmdb)。

地理位置在日志行建立索引时写入。安装或更换数据库后，需要重建对应日志的索引，已索引的日志行才会使用新的数据。

## IndexingEnabled

- 类型：`boolean`
- 默认值：`false`
- 环境变量：`NGINX_UI_NGINX_LOG_INDEXING_ENABLED`
- 版本：`>= v2.2.0`

开启日志索引及上述分析功能。关闭时，日志列表只显示检测到的日志文件及其基本信息，日志页面只有原始视图。实时跟踪日志不受此项影响。

## IndexPath

- 类型：`string`
- 默认值：`app.ini` 所在目录下的 `log-index`
- 环境变量：`NGINX_UI_NGINX_LOG_INDEX_PATH`
- 版本：`>= v2.2.0`

索引所在目录，请使用绝对路径。如果无法确定 `app.ini` 所在目录，则使用工作目录下的 `./log-index`。

## IncrementalIndexInterval

- 类型：`int`（分钟）
- 默认值：`15`
- 环境变量：`NGINX_UI_NGINX_LOG_INCREMENTAL_INDEX_INTERVAL`
- 版本：`>= v2.2.0`

Nginx UI 检查日志新增内容的频率。数值越小，分析数据越接近实时，后台 CPU 占用也越高。配置为 `0` 或负数时使用默认的 15 分钟。

## MaxConcurrentIndexTasks

- 类型：`int`
- 默认值：`0`（自动计算）
- 环境变量：`NGINX_UI_NGINX_LOG_MAX_CONCURRENT_INDEX_TASKS`
- 版本：`>= v2.5.7`

限制同时建立索引的日志组数量，包括首次索引、重启后的恢复任务以及增量索引任务。每个正在索引的日志组都会在内存中持有解析批次，因此该值是控制索引期间内存峰值的主要手段。

配置为 `0` 或负数**并不**表示不限制。此时上限根据进程可用的 CPU 数量计算（如设置了容器 CPU 配额，则以配额为准）：取其一半，最少为 `1`，最多为 `2`。配置为正数可覆盖该值，例如在索引与 Nginx 争抢内存的小规格主机上设为 `1`。

## IndexCustomMMDB

- 类型：`string`
- 默认值：空（使用 GeoLite2 City 数据库）
- 环境变量：`NGINX_UI_NGINX_LOG_INDEX_CUSTOM_MMDB`

指定自定义 MaxMind DB（`.mmdb`）文件的路径，用于 IP 地理位置解析。自定义记录可以提供国家、省份、城市及四个业务标签（`c1` 至 `c4`），例如分公司、工厂、部门和网络类型。

绝对路径直接使用配置值。相对路径以当前使用的 `app.ini` 所在目录为基准解析，而非进程的工作目录。例如，将 `enterprise.mmdb` 放在 `app.ini` 同一目录下，并配置：

```ini
[nginx_log]
IndexingEnabled = true
IndexCustomMMDB = enterprise.mmdb
```

也可以通过环境变量指定 Nginx UI 进程可访问的路径：

```bash
NGINX_UI_NGINX_LOG_INDEX_CUSTOM_MMDB=/etc/nginx-ui/enterprise.mmdb
```

使用 Docker 部署时，需要将数据库挂载到容器内，并填写容器内的路径。运行 Nginx UI 的用户必须具有该文件的读取权限。

::: warning 数据库选择规则
如果 `app.ini` 同一目录下存在 `GeoLite2-City.mmdb`，它的优先级高于 `IndexCustomMMDB`。要使用自定义数据库，请先将标准数据库移至备份位置。两个数据库不会合并；未匹配自定义 IP 范围的地址也不会回退到标准城市数据库查询。

配置路径不会自动下载或生成文件。
:::

设置了 `IndexCustomMMDB` 时，**偏好设置 > GeoLite** 会显示所配置的自定义数据库，并隐藏 GeoLite2 数据库的下载操作。

### 生成自定义数据库

仓库的 [template/custom-mmdb](https://github.com/0xJacky/nginx-ui/tree/dev/template/custom-mmdb) 目录提供了生成脚本和示例数据。

1. 准备 Python 3 环境及生成脚本所需的依赖：`mmdb_writer` 和 `netaddr`。
2. 编辑 `region_codes.json`，定义国家、省份和城市的层级关系。附带的文件仅作为起始模板；引用尚未列出的地区前，请先补充对应数据。
3. 编辑 `ip_inventory.json`，将单个 IPv4 地址或 CIDR 网段映射到上述地区层级及业务标签。四个标签键均需保留，未使用的标签填写空字符串。

例如，苏州某网段的清单条目可以写为：

```json
{
  "10.10.0.0/16": {
    "country": "CN",
    "province": "320000",
    "city": "320500",
    "c1": "苏州分公司",
    "c2": "工厂 A",
    "c3": "生产 IT 部门",
    "c4": "有线网络"
  }
}
```

在仓库根目录运行生成脚本：

```bash
python3 template/custom-mmdb/Build_Custom_mmdb.py
```

脚本会校验网络地址和地区引用，然后在 `template/custom-mmdb` 中生成 `enterprise.mmdb` 及清单导出文件 `enterprise_data.json`。附带的生成脚本创建的是 IPv4 数据库，单个 IPv4 地址会转换为 `/32` 网段。

将 `enterprise.mmdb` 复制到配置指定的位置，并在修改配置或替换数据库后重启 Nginx UI。然后重建需要使用新地理信息和业务标签的日志索引。

## GeoMapPath

- 类型：`string`
- 默认值：空（即 `app.ini` 所在目录下的 `maps`）
- 环境变量：`NGINX_UI_NGINX_LOG_GEO_MAP_PATH`

地图使用的中国及省级边界 GeoJSON 文件所在目录，文件名需遵循 `100000_full.json`、`<省级adcode>_full.json` 的命名规则。默认只提供世界地图，边界文件需要自行下载并放入该目录。

- 配置为绝对路径时，直接使用该路径。
- 配置为相对路径时，以 `app.ini` 所在目录为基准解析。
- 没有本地文件时，仪表盘在中文语言下仍可展示中国地图，边界数据会从 CDN 加载。

示例：

```ini
[nginx_log]
GeoMapPath = /etc/nginx-ui/maps
```
