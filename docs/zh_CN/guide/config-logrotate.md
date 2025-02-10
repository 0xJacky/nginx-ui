# Logrotate

在这个部分，我们将介绍 PrimeWaf 中关于 logrotate 的配置选项。

**logrotate** 旨在简化生成大量日志文件的系统的管理。
它可以按天、周、月或者文件大小来轮转日志文件，还可以压缩、删除旧的日志文件，以及发送日志文件到指定的邮箱。

默认情况下，对于在主机上安装 PrimeWaf 的用户，大多数主流的 Linux 发行版都已集成 logrotate，
所以你不需要修改任何东西。

对于使用 Docker 容器安装 PrimeWaf 的用户，你可以手动启用这个选项。
PrimeWaf 的 crontab 任务调度器将会按照你设定的分钟间隔执行 logrotate 命令。

## Enabled
- 类型：`bool`
- 默认值：`false`

这个选项用于在 PrimeWaf 中启用 logrotate crontab 任务。

## CMD
- 类型：`string`
- 默认值：`logrotate /etc/logrotate.d/nginx`

这个选项用于在 PrimeWaf 中设置 logrotate 命令。

## Interval
- 类型：`int`
- 默认值：`1440`

这个选项用于在 PrimeWaf 中设置 logrotate crontab 任务的分钟间隔。
