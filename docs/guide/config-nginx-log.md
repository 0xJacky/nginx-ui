# Nginx Log

This section covers configuration options for Nginx log processing and analysis features in Nginx UI.

## Advanced Indexing

### AdvancedIndexingEnabled

- Type: `boolean`
- Default: `false`
- Environment Variable: `NGINX_UI_NGINX_LOG_ADVANCED_INDEXING_ENABLED`
- Version: `>= v2.2.0`

This option enables advanced indexing for Nginx logs, which provides high-performance log search and analysis capabilities.

### System Requirements

#### Minimum Requirements
- **CPU**: 1 core minimum
- **Memory**: 2GB RAM minimum
- **Storage**: At least 20GB available disk space

#### Recommended Configuration
- **CPU**: 2+ cores recommended
- **Memory**: 4GB+ RAM recommended
- **Storage**: SSD storage for better I/O performance

### Performance Metrics

Based on production validation and comprehensive testing (M2 Pro 12 cores, September 2025):

| Metric | Value | Description |
|--------|-------|-------------|
| **Production Pipeline** | **~10,000 records/sec** | Complete indexing with search capabilities |
| **Parser Performance** | **~932K records/sec** | Stream processing only |
| **CPU Utilization** | **90%+** | Optimized multi-core processing |
| **Memory Efficiency** | **Zero-allocation design** | Advanced memory pooling system |
| **Adaptive Scaling** | **12→36 workers** | Dynamic resource optimization |
| **Batch Optimization** | **1000→6000** | Real-time throughput tuning |

### Features

When advanced indexing is enabled, you get access to the following features:

#### Core Capabilities
- **Zero-allocation pipeline** - Optimized memory usage for high-performance processing
- **Dynamic shard management** - Intelligent distribution of log data across shards
- **Incremental index scanning** - Only indexes new log entries for efficiency
- **Automated log rotation detection** - Seamlessly handles rotated log files

#### Search & Analysis
- **Advanced search & filtering** - Complex queries with multiple criteria
- **Full-text search with regex support** - Powerful pattern matching capabilities
- **Cross-file timeline correlation** - Analyze events across multiple log files
- **Error pattern recognition** - Automatic detection of error patterns

#### Data Processing
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