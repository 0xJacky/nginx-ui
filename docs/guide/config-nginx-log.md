# Nginx Log

This section covers configuration options for Nginx log processing and analysis features in Nginx UI.

## Indexing

### IndexingEnabled

- Type: `boolean`
- Default: `false`
- Environment Variable: `NGINX_UI_NGINX_LOG_INDEXING_ENABLED`
- Version: `>= v2.2.0`

This option enables indexing for Nginx logs, which provides high-performance log search and analysis capabilities.

#### Behavior When Disabled (Basic Mode)

When `IndexingEnabled` is set to `false`, Nginx UI still discovers log entries from your Nginx configuration and shows them in the Logs list. In this basic mode:

- You can view the list of detected log files (grouped by simple rotation patterns), but advanced features like indexing metrics, document counts, and search shards are not available.
- Real-time viewing (tail) continues to work based on resolved access/error log paths.

### IndexPath

- Type: `string`
- Version: `>= v2.2.0`

- By default, Bleve index files are stored in the `log-index` directory located under your Nginx UI config directory (for example, `/usr/local/nginx-ui/log-index`).
- If the config directory cannot be determined, the fallback path is `./log-index` relative to the application.

### IncrementalIndexInterval

- Type: `int` (minutes)
- Default: `15` when the value is `0` or negative
- Version: `>= v2.2.0`

Controls how frequently the incremental indexing job scans access logs for new entries. Lower values keep analytics closer to real time but increase background CPU usage; higher values reduce CPU load at the cost of staler analytics data. Set `0` or a negative value to use the safe default of 15 minutes.

### IndexCustomMMDB

- Type: `string`
- Default: empty (use the standard GeoLite2 database)
- Environment Variable: `NGINX_UI_NGINX_LOG_INDEX_CUSTOM_MMDB`
- Requires a build that includes [PR #1843](https://github.com/0xJacky/nginx-ui/pull/1843).

Sets the path to a custom MaxMind DB (`.mmdb`) file for GeoIP enrichment during log indexing. Custom records can supply country, province, city, and four business labels (`c1` through `c4`), such as branch, factory, department, and network type. Enable `IndexingEnabled` to use indexed log analytics.

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

When the standard file is absent, a missing or invalid custom file prevents the GeoIP database from loading. Configuring a path does not download or generate the file.
:::

#### Build a custom database

The repository includes a generator and sample data in [template/custom-mmdb](https://github.com/0xJacky/nginx-ui/tree/dev/template/custom-mmdb). Use these files from a checkout containing PR #1843.

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

Copy `enterprise.mmdb` to the configured location and restart Nginx UI after changing the setting or replacing the database. GeoIP fields are stored during indexing, so existing indexed entries need to be reindexed to reflect the new geographic data and business labels.

The GeoLite2 settings page displays the configured custom database filename and hides the re-download action while `IndexCustomMMDB` is nonempty. This indicator reflects the configured path; database selection still follows the precedence described above.

## System Requirements

### Minimum Requirements
- **CPU**: 1 core minimum
- **Memory**: 2GB RAM minimum
- **Storage**: At least 20GB available disk space

### Recommended Configuration
- **CPU**: 2+ cores recommended
- **Memory**: 4GB+ RAM recommended
- **Storage**: SSD storage for better I/O performance

## Performance Metrics

Based on production validation and comprehensive testing (M2 Pro 12 cores, September 2025):

| Metric | Value | Description |
|--------|-------|-------------|
| **Production Pipeline** | **~10,000 records/sec** | Complete indexing with search capabilities |
| **Parser Performance** | **~932K records/sec** | Stream processing only |
| **CPU Utilization** | **90%+** | Optimized multi-core processing |
| **Memory Efficiency** | **Zero-allocation design** | Advanced memory pooling system |
| **Adaptive Scaling** | **12→36 workers** | Dynamic resource optimization |
| **Batch Optimization** | **1000→6000** | Real-time throughput tuning |

## Features

When advanced indexing is enabled, you get access to the following features:

### Core Capabilities
- **Zero-allocation pipeline** - Optimized memory usage for high-performance processing
- **Dynamic shard management** - Intelligent distribution of log data across shards
- **Incremental index scanning** - Only indexes new log entries for efficiency
- **Automated log rotation detection** - Seamlessly handles rotated log files

### Search & Analysis
- **Advanced search & filtering** - Complex queries with multiple criteria
- **Full-text search with regex support** - Powerful pattern matching capabilities
- **Cross-file timeline correlation** - Analyze events across multiple log files
- **Error pattern recognition** - Automatic detection of error patterns

### Data Processing
- **Compressed log file support** - Works with gzipped and other compressed formats
- **Offline GeoIP analysis** - Location-based analytics without external services
- **Real-time analytics dashboard** - Live monitoring and statistics
- **Multi-dimensional data visualization** - Advanced charts and graphs

### Usage Considerations

::: tip Performance Impact Notice
Advanced indexing delivers enterprise-grade performance with **~10,000 records/second** throughput for complete log processing. The system automatically optimizes CPU utilization (90%+) and adapts worker scaling (12→36) for optimal performance based on your hardware.
:::

::: info Open Source Limitation
- Advanced log indexing features are free and open source for all users
- We do not accept feature requests for this functionality
- For commercial or professional use, contact business@uozi.com
:::

::: warning Initial Indexing
When you enable advanced indexing, the system will immediately start indexing existing log files. This initial indexing process may temporarily impact system performance.
:::

