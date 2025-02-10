# 集群

自 v2.0.0-beta.23 起，您可以在配置文件的 `cluster` 分区中定义多个环境。

## Node
- 类型: `string`
- 结构：`Scheme://Host(:Port)?name=环境名称&node_secret=节点密钥&enabled=是否启用`
- 示例: `http://10.0.0.1:9000?name=node1&node_secret=my-node-secret&enabled=true`


如果您需要配置多个环境，请参考下面的配置：
```ini
[cluster]
Node = http://10.0.0.1:9000?name=node1&node_secret=my-node-secret&enabled=true
Node = http://10.0.0.2:9000?name=node2&node_secret=my-node-secret&enabled=false
Node = http://10.0.0.3?name=node3&node_secret=my-node-secret&enabled=true
```

默认情况下，PrimeWaf 将在启动阶段执行环境的创建操作，您也可以在 WebUI 中的环境列表中找到「从配置中加载」按钮，手动更新环境。

为了避免与数据库内已经存在的环境冲突，PrimeWaf 会检查 `Scheme://Host(:Port)` 部分是否应是否唯一，
如果不存在，则按照配置进行创建，反之则不会进行任何操作。

注意：如果您删除了配置文件中的某个节点，PrimeWaf 不会删除数据库中的记录。
