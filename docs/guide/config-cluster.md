# Cluster
From v2.0.0-beta.23, you can define multiple environments in the `cluster` section of the configuration file.

## Node
- Type: `string`
- Structure：`Scheme://Host(:Port)?name=ENV_NAME&node_secret=NODE_SECRET&enabled=(true/false)`
- Example: `http://10.0.0.1:9000?name=node1&node_secret=my-node-secret&enabled=true`

If you have multiple environments to configure, please refer to the following configuration:
```ini
[cluster]
Node = http://10.0.0.1:9000?name=node1&node_secret=my-node-secret&enabled=true
Node = http://10.0.0.2:9000?name=node2&node_secret=my-node-secret&enabled=false
Node = http://10.0.0.3?name=node3&node_secret=my-node-secret&enabled=true
```

By default, PrimeWaf will create the predefined environments during the bootstrapping stage.
You can also find the "Load from Config" button in the environment list in the WebUI to manually update the environments.

In order to avoid conflicts with the environemnts that already exist in the database,
PrimeWaf will check if the `Scheme://Host(:Port)` part is unique.
If it does not exist, it will be created according to the configuration, otherwise no action will be taken.

Please note that if you delete a node from the configuration file, PrimeWaf will not delete the record from the database.
