# Nginx Log

Nginx UI 会列出从 Nginx 配置中发现的日志文件，以及 Nginx 默认的访问日志和错误日志。你可以分页查看日志，也可以实时跟踪。这些功能无需任何设置。

## 日志分析插件

结构化搜索、流量面板、访客地图和 IP 地理库由官方插件 **日志分析**（`com.nginxui.log-analytics`）提供。它们以前以「高级索引」的形式内置在 Nginx UI 中。

- 在插件页面安装并启用该插件。没有外网的节点，可以在同一页面上传插件安装包。
- 启用后会自动开始索引。停用插件即停止索引，并释放它占用的全部内存。Nginx UI 本身不会加载这部分代码。
- 没有安装插件时，日志列表只显示基础列，日志页面只有原始视图。
- 集群中，每个需要分析日志的节点都要安装该插件。

### 设置

原来 `[nginx_log]` 中的 `IncrementalIndexInterval`、`MaxConcurrentIndexTasks`、`IndexCustomMMDB` 和 `GeoMapPath` 不再由 Nginx UI 读取，它们现在是插件的设置，在插件页面的插件设置中修改：

| 插件设置 | 原来的配置 |
|----------|------------|
| 索引间隔（分钟） | `IncrementalIndexInterval` |
| 同时索引的日志数 | `MaxConcurrentIndexTasks` |
| 自定义 IP 地理库 | `IndexCustomMMDB` |
| 地图文件目录 | `GeoMapPath` |

IP 地理库（GeoLite2）也在插件设置中下载。[template/custom-mmdb](https://github.com/0xJacky/nginx-ui/tree/dev/template/custom-mmdb) 中用于生成自定义库的脚本仍然适用，把自定义库的设置指向生成的文件即可。

### 从高级索引升级

如果之前开启了 `IndexingEnabled`，日志页面会提示「日志分析已改为插件」，从提示中前往安装即可。插件首次启动时会接管已有的索引及其记录、上述设置和已下载的 IP 地理库，不会重复索引。之后 Nginx UI 会把 `IndexingEnabled` 关闭，并删除旧的索引数据表。

`IndexingEnabled` 和 `IndexPath` 仍保留在 `app.ini` 中，也仍可用 `NGINX_UI_NGINX_LOG_INDEXING_ENABLED` 和 `NGINX_UI_NGINX_LOG_INDEX_PATH` 设置。它们只用于这次交接，网页界面不能再修改。
