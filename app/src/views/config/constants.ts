import {createEnum} from '@/lib/helper'

// refer to https://nginx.org/en/docs/ngx_core_module.html#error_log
// nginx log level: debug, info, notice, warn, error, crit, alert, or emerg

const logLevel = createEnum({
  Debug: [0, 'debug'],
  Info: [1, 'info'],
  Notice: [2, 'notice'],
  Warn: [3, 'warn'],
  Error: [4, 'error'],
  Crit: [5, 'crit'],
  Alert: [6, 'alert'],
  Emerg: [7, 'emerg']
})

export default logLevel
