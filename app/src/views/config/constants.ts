// refer to https://nginx.org/en/docs/ngx_core_module.html#error_log
// nginx log level: debug, info, notice, warn, error, crit, alert, or emerg

export enum logLevel {
  Debug,
  Info,
  Notice,
  Warn,
  Error,
  Crit,
  Alert,
  Emerg,
}
