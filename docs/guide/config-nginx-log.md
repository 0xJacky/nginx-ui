# Nginx Log

Nginx UI lists the log files it finds in your Nginx configuration, together with the default access and error logs of Nginx. You can read a log page by page or follow it live. This needs no setup.

## Log analytics plugin

Structured search, the traffic dashboard, visitor maps and the IP location database are provided by the official **Log Analytics** plugin (`com.nginxui.log-analytics`). They used to be built into Nginx UI as "advanced indexing".

- Install the plugin from the Plugins page and enable it. On a node without internet access, upload the plugin package on the same page.
- Indexing starts on its own. Disabling the plugin stops indexing and releases all the memory it used. Nginx UI itself loads none of this code.
- Without the plugin, the log list shows the basic columns only and the log page has the raw view only.
- In a cluster, install the plugin on every node that should analyse its logs.

### Settings

The former `[nginx_log]` options `IncrementalIndexInterval`, `MaxConcurrentIndexTasks`, `IndexCustomMMDB` and `GeoMapPath` are no longer read by Nginx UI. They are settings of the plugin now, open them from the plugin settings on the Plugins page:

| Plugin setting | Former option |
|----------------|---------------|
| Indexing interval (minutes) | `IncrementalIndexInterval` |
| Logs indexed at once | `MaxConcurrentIndexTasks` |
| Custom IP location database | `IndexCustomMMDB` |
| Map files folder | `GeoMapPath` |

The IP location database (GeoLite2) is downloaded from the plugin settings as well. The generator for a custom database in [template/custom-mmdb](https://github.com/0xJacky/nginx-ui/tree/dev/template/custom-mmdb) works with the plugin, point the custom database setting at the generated file.

### Upgrading from advanced indexing

If `IndexingEnabled` was on, the log pages show a notice that log analytics is now a plugin. Install the plugin from there. The first time it starts, the plugin takes over the existing index and its records, the settings above and the downloaded IP location database, so nothing is indexed twice. Nginx UI then switches `IndexingEnabled` off and removes its old index table.

`IndexingEnabled` and `IndexPath` stay in `app.ini` and can still be set with `NGINX_UI_NGINX_LOG_INDEXING_ENABLED` and `NGINX_UI_NGINX_LOG_INDEX_PATH`. They are only read for this handoff and cannot be changed in the web interface anymore.
