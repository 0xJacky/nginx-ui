# Upstream Availability Monitoring Refactor

## 概述

重构了upstream可用性检测功能，从原来的WebSocket实时检测改为后台定时任务方式，提供更高效和稳定的监控。

## 主要改动

### 1. 后台任务机制
- 通过 `cache.RegisterCallback` 注册 `ParseProxyTargetsFromRawContent` 自动收集所有配置文件中的代理目标
- 创建了 `UpstreamService` 单例服务管理所有代理目标和检测结果
- 实现了去重机制，避免重复检测相同的地址
- 支持跟踪每个代理目标的来源配置文件

### 2. 定时检测
- 添加了cron job，默认每30秒执行一次可用性检测
- 当有WebSocket连接时，检测频率自动提高到每5秒一次
- 实现了并发控制，防止多个检测任务同时运行

### 3. API接口
保留了两个简洁的接口：

#### HTTP GET接口
- 路径：`/api/upstream/availability`
- 功能：获取所有upstream的缓存监控结果
- 返回数据：
  ```json
  {
    "results": {
      "127.0.0.1:8080": {
        "online": true,
        "latency": 1.23
      }
    },
    "targets": [...],
    "last_update_time": "2024-01-01T00:00:00Z",
    "target_count": 10
  }
  ```

#### WebSocket接口
- 路径：`/api/upstream/availability/ws`
- 功能：实时推送监控结果
- 特点：
  - 连接时立即发送当前结果
  - 每5秒推送一次最新结果
  - 自动管理连接数，优化检测频率

## 技术细节

### 去重机制
- 基于 `host:port` 作为唯一标识
- 支持多个配置文件引用同一个代理目标
- 当某个配置文件被删除时，只有独占的代理目标会被移除

### 并发控制
- 使用互斥锁防止多个检测任务同时运行
- WebSocket连接计数器管理检测频率
- 后台任务会检查是否有活跃的WebSocket连接，避免重复检测

### 性能优化
- 缓存检测结果，减少实时检测的开销
- 批量检测所有目标，提高效率
- 使用goroutine池限制并发连接数（MaxConcurrentWorker = 10）

## 使用示例

### 获取监控结果
```bash
curl http://localhost:9000/api/upstream/availability
```

### WebSocket连接
```javascript
const ws = new WebSocket('ws://localhost:9000/api/upstream/availability/ws');
ws.onmessage = (event) => {
  const results = JSON.parse(event.data);
  console.log('Upstream status:', results);
};
``` 