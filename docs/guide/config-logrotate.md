# Logrotate

In this section, we will introduce configuration options in Nginx UI about logrotate.

**logrotate** is designed to ease administration of systems that generate large numbers of log files.
It allows automatic rotation, compression, removal, and mailing of log files.
Each log file may be handled daily, weekly, monthly, or when it grows too large.

By default, logrotate is enabled in most mainstream Linux distributions for users who install Nginx UI on the host machine,
so you don't need to modify anything.

For users who install Nginx UI using Docker containers, you can manually enable this option.
The crontab task scheduler of Nginx UI will execute the logrotate command at the interval you set in minutes.

## Enabled
- Type: `bool`
- Default: `false`

This option is used to enable logrotate crontab task in Nginx UI.

## CMD
- Type: `string`
- Default: `logrotate /etc/logrotate.d/nginx`

This option is used to set the logrotate command in Nginx UI.

## Interval
- Type: `int`
- Default: `1440`

This option is used to set the interval in minutes of logrotate crontab task in Nginx UI.
