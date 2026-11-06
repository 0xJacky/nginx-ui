# Nginx Log

Nginx UI lists the log files it finds in your Nginx configuration, together with the default access and error logs of Nginx. You can read a log page by page or follow it live. This needs no setup.

## Log Analytics

Log analytics is built into Nginx UI. Once indexing is enabled, Nginx UI indexes the access and error logs it lists and offers:

- structured search with filters, a time range and paged results, for access and error logs;
- a traffic dashboard with requests, visitors, status codes, top pages, browsers, operating systems and devices;
- visitor maps by country, and by province and city when the GeoLite2 City database is installed.

Enable indexing with **Enable Advanced Indexing** on the log list, or with [IndexingEnabled](#indexingenabled). Nginx UI then indexes the existing logs and, after that, reads only what was appended. Rotated files, including files renamed by logrotate, files copied by `copytruncate` and files compressed to `.gz`, are recognised by their content and are not indexed twice.

The index is a column store kept in the `logstore` directory inside [IndexPath](#indexpath). It is usually much smaller than the raw logs.

::: tip Upgrading from v2
v2 used a Bleve index. The first start of v3 removes the old index directories and indexes every log again into the new store. Nothing has to be done, but the rebuild takes a while on large logs.
:::

### Search syntax

The search box combines words and `field:value` filters with AND. A leading `-` excludes a term, and double quotes keep a phrase together. Plain words match the raw log line as case-insensitive substrings.

| Filter | Example |
|--------|---------|
| Status code, class or range | `status:404`, `status:5xx`, `status:400-499`, `status:>=500` |
| Method | `method:POST` |
| Client address or network | `ip:10.0.0.0/8`, `ip:2001:db8::/32` |
| Path, user agent, referer (substring) | `path:/api/`, `ua:curl`, `referer:google` |
| Browser, OS, device, region, city (substring) | `browser:firefox`, `os:android`, `city:berlin` |
| Country code | `country:CN` |
| Bytes sent | `bytes:>1000` |
| Request time in seconds | `rt:>0.5`, `rt:0.1..0.5` |
| Error log level | `level:error`, `-level:warn` |

A term that looks like a filter but is not one is searched as text, and the log page shows a warning for it.

### IP location

Countries are resolved from the GeoLite2 Country database embedded in Nginx UI, so the world map works without any download. Provinces and cities need the GeoLite2 City database, which you download from **Preferences > GeoLite**. It is saved as `GeoLite2-City.mmdb` next to `app.ini`. You can also use a custom database, see [IndexCustomMMDB](#indexcustommmdb).

Location is stored while a line is indexed. After you install or change a database, rebuild the index of a log to apply it to lines that were already indexed.

## IndexingEnabled

- Type: `boolean`
- Default: `false`
- Environment Variable: `NGINX_UI_NGINX_LOG_INDEXING_ENABLED`
- Version: `>= v2.2.0`

Enables log indexing and the analytics features above. When it is off, the log list shows the detected log files with basic information, and the log page shows the raw log only. Following a log live works either way.

## IndexPath

- Type: `string`
- Default: `log-index` in the directory that holds `app.ini`
- Environment Variable: `NGINX_UI_NGINX_LOG_INDEX_PATH`
- Version: `>= v2.2.0`

The directory of the index. Use an absolute path. If the directory of `app.ini` cannot be determined, `./log-index` relative to the working directory is used.

## IncrementalIndexInterval

- Type: `int` (minutes)
- Default: `15`
- Environment Variable: `NGINX_UI_NGINX_LOG_INCREMENTAL_INDEX_INTERVAL`
- Version: `>= v2.2.0`

How often Nginx UI checks the logs for new lines. Lower values keep the analytics closer to real time and use more CPU in the background. `0` or a negative value uses the default of 15 minutes.

## MaxConcurrentIndexTasks

- Type: `int`
- Default: `0` (sized automatically)
- Environment Variable: `NGINX_UI_NGINX_LOG_MAX_CONCURRENT_INDEX_TASKS`
- Version: `>= v2.5.7`

Caps how many log groups are indexed at the same time, by the initial indexing, the recovery after a restart and the incremental job. Each log group being indexed holds its parse batches in memory, so this is the main lever on peak memory during indexing.

`0` or a negative value does **not** mean unlimited. The limit is then sized from the CPUs the process is allowed to use (the container's CPU quota when one is set): half of them, at least `1` and at most `2`. Set a positive value to override it, for example `1` on a small host where indexing competes with Nginx for memory.

## IndexCustomMMDB

- Type: `string`
- Default: empty (use the GeoLite2 City database)
- Environment Variable: `NGINX_UI_NGINX_LOG_INDEX_CUSTOM_MMDB`

Sets the path to a custom MaxMind DB (`.mmdb`) file for IP location. Custom records can supply country, province, city, and four business labels (`c1` through `c4`), such as branch, factory, department, and network type.

Absolute paths are used as configured. Relative paths are resolved against the directory containing the active `app.ini`, not the process working directory. For example, place `enterprise.mmdb` beside `app.ini` and configure:

```ini
[nginx_log]
IndexingEnabled = true
IndexCustomMMDB = enterprise.mmdb
```

Alternatively, set the environment variable to a path visible to the Nginx UI process:

```bash
NGINX_UI_NGINX_LOG_INDEX_CUSTOM_MMDB=/etc/nginx-ui/enterprise.mmdb
```

For Docker deployments, mount the database into the container and use its container path. The file must be readable by the user running Nginx UI.

::: warning Database selection
If `GeoLite2-City.mmdb` exists beside `app.ini`, it takes precedence over `IndexCustomMMDB`. To use the custom database, move the standard database to a backup location first. The two databases are not merged, and unmatched custom IP ranges do not fall back to the standard city database.

Configuring a path does not download or generate the file.
:::

Preferences > GeoLite shows the configured custom database and hides the download of the GeoLite2 database while `IndexCustomMMDB` is set.

### Build a custom database

The repository includes a generator and sample data in [template/custom-mmdb](https://github.com/0xJacky/nginx-ui/tree/dev/template/custom-mmdb).

1. Prepare a Python 3 environment with the generator's dependencies: `mmdb_writer` and `netaddr`.
2. Edit `region_codes.json` to define the country, province, and city hierarchy. The supplied file is a starting template; add any missing regions before referencing them.
3. Edit `ip_inventory.json` to map individual IPv4 addresses or CIDR ranges to that hierarchy and your business labels. Include all four label keys; use an empty string for unused labels.

For example, an inventory entry for a network in Suzhou is:

```json
{
  "10.10.0.0/16": {
    "country": "CN",
    "province": "320000",
    "city": "320500",
    "c1": "Suzhou branch",
    "c2": "Factory A",
    "c3": "Production IT",
    "c4": "Wired network"
  }
}
```

Run the generator from the repository root:

```bash
python3 template/custom-mmdb/Build_Custom_mmdb.py
```

The script validates the network addresses and region references, then writes `enterprise.mmdb` and an inventory export, `enterprise_data.json`, to `template/custom-mmdb`. The supplied generator creates an IPv4 database; individual IPv4 addresses become `/32` networks.

Copy `enterprise.mmdb` to the configured location and restart Nginx UI after changing the setting or replacing the database. Then rebuild the index of the logs that should carry the new locations and labels.

## GeoMapPath

- Type: `string`
- Default: empty (`maps` in the directory that holds `app.ini`)
- Environment Variable: `NGINX_UI_NGINX_LOG_GEO_MAP_PATH`

The directory of the China and province boundary GeoJSON files used by the maps. File names follow `100000_full.json` and `<province_adcode>_full.json`. Only the world map is provided by default; download the boundary files yourself and place them in this directory.

- An absolute path is used directly.
- A relative path is resolved against the directory containing `app.ini`.
- The dashboard can still show the China map in Chinese locales without local files, the boundaries are then loaded from a CDN.

Example:

```ini
[nginx_log]
GeoMapPath = /etc/nginx-ui/maps
```
