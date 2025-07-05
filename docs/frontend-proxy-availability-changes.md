# Frontend Proxy Availability Changes

## 概述

根据后端API重构，简化了前端的代理可用性监控机制。

## 主要变更

### 1. API文件更新 (`app/src/api/upstream.ts`)
- 新增 `getAvailability()` HTTP GET方法获取所有可用性状态
- 新增 `availabilityWebSocket()` WebSocket方法实时更新状态
- 更新了接口类型定义 `UpstreamAvailabilityResponse`

### 2. Store重构 (`app/src/pinia/moudule/proxyAvailability.ts`)

#### 移除的功能：
- 复杂的组件注册/注销机制 (`registerComponent`, `unregisterComponent`)
- 基于组件的目标管理
- 向后端发送目标列表的逻辑
- 防抖处理

#### 新增的功能：
- `initialize()` - 从HTTP GET接口初始化状态
- `startMonitoring()` - 启动完整监控（HTTP初始化 + WebSocket连接）
- `stopMonitoring()` - 停止监控并清理连接
- `hasAvailabilityData()` - 检查目标是否有可用性数据
- `getAllTargets()` - 获取所有可用目标列表

#### 简化的功能：
- `getAvailabilityResult()` - 简单地从缓存获取结果
- 自动WebSocket连接管理
- 页面卸载时自动清理

### 3. 组件使用方式

组件现在只需要：
```typescript
const proxyStore = useProxyAvailabilityStore()

// 获取可用性结果（从缓存）
const result = proxyStore.getAvailabilityResult(target)
```

不再需要注册/注销组件或发送目标到后端。

### 4. 布局初始化 (`app/src/layouts/BaseLayout.vue`)

在用户登录后的主布局中初始化监控，确保用户已认证：
```typescript
const proxyAvailabilityStore = useProxyAvailabilityStore()

onMounted(() => {
  // Start monitoring for upstream availability
  proxyAvailabilityStore.startMonitoring()
})

onUnmounted(() => {
  // Stop monitoring when layout is unmounted
  proxyAvailabilityStore.stopMonitoring()
})
```

## 工作流程

1. **用户登录**：用户成功登录并进入主布局
2. **布局挂载**：`BaseLayout.vue` 挂载时自动调用 `startMonitoring()`
3. **初始化**：通过HTTP GET获取当前所有可用性状态
4. **实时更新**：建立WebSocket连接，接收实时状态更新
5. **组件使用**：组件直接从缓存读取状态，无需额外请求
6. **自动清理**：布局卸载时自动断开WebSocket连接

## 优势

- **简化架构**：移除复杂的组件管理逻辑
- **更好性能**：减少不必要的网络请求
- **统一数据源**：所有组件共享同一份缓存数据
- **自动管理**：无需手动处理连接生命周期 