import type { PreferenceSectionKey } from './sections'

export interface SettingCatalogEntry {
  // Dot path inside the settings object. It is also the SettingRow path.
  path: string
  section: PreferenceSectionKey
  title: string
  description?: string
  // Panel name, only set where the same title appears twice in one section.
  panel?: string
}

// Titles and descriptions mirror the SettingRow props in the tab components,
// so the search box and the save bar show the same words as the page.
// Keep both in sync when a row is renamed.
export function buildSettingCatalog(): SettingCatalogEntry[] {
  return [
    // Server
    { path: 'server.host', section: 'server', title: $gettext('Host') },
    { path: 'server.port', section: 'server', title: $gettext('Port') },
    { path: 'listener.unix_socket', section: 'server', title: $gettext('Unix Socket'), description: $gettext('Nginx UI listens on this Unix socket instead of the TCP host and port.') },
    { path: 'server.run_mode', section: 'server', title: $gettext('Run Mode') },
    { path: 'server.enable_https', section: 'server', title: $gettext('Enable HTTPS'), description: $gettext('Serves the web interface over HTTPS. Saving this change restarts Nginx UI and reloads the page.') },
    { path: 'server.ssl_cert', section: 'server', title: $gettext('SSL Certificate Path') },
    { path: 'server.ssl_key', section: 'server', title: $gettext('SSL Key Path') },
    { path: 'server.enable_h2', section: 'server', title: $gettext('Enable HTTP/2'), description: $gettext('Enables HTTP/2 support with multiplexing and server push capabilities') },
    { path: 'server.enable_h3', section: 'server', title: $gettext('Enable HTTP/3'), description: $gettext('Enables HTTP/3 support based on QUIC protocol for best performance') },

    // App
    { path: 'app.jwt_secret', section: 'app', title: $gettext('Jwt Secret') },
    { path: 'app.page_size', section: 'app', title: $gettext('Page Size') },

    // Node
    { path: 'node.secret', section: 'node', title: $gettext('Node Secret') },
    { path: 'node.instance_id', section: 'node', title: $gettext('Instance ID') },
    { path: 'node.name', section: 'node', title: $gettext('Node name'), description: $gettext('Customize the name of local node to be displayed in the environment indicator.') },
    { path: 'node.skip_installation', section: 'node', title: $gettext('Skip Installation') },
    { path: 'node.demo', section: 'node', title: $gettext('Demo') },
    { path: 'node.icp_number', section: 'node', title: $gettext('ICP Number'), description: $gettext('Shown in the page footer. Only needed for sites hosted in mainland China.') },
    { path: 'node.public_security_number', section: 'node', title: $gettext('Public Security Number'), description: $gettext('Shown in the page footer. Only needed for sites hosted in mainland China.') },

    // HTTP
    { path: 'http.github_proxy', section: 'http', title: $gettext('Github Proxy'), description: $gettext('Used for downloads from GitHub, such as upgrades and the GeoLite database.') },
    { path: 'http.http_proxy', section: 'http', title: $gettext('HTTP Proxy'), description: $gettext('Used for other outgoing requests made by Nginx UI.') },
    { path: 'http.insecure_skip_verify', section: 'http', title: $gettext('Insecure Skip Verify') },

    // Auth
    { path: 'webauthn.rpid', section: 'auth', title: $gettext('RPID') },
    { path: 'webauthn.rp_display_name', section: 'auth', title: $gettext('RP Display Name') },
    { path: 'webauthn.rp_origins', section: 'auth', title: $gettext('RP Origins') },
    { path: 'auth.ban_threshold_minutes', section: 'auth', title: $gettext('Ban Threshold Minutes'), description: $gettext('Window in which failed sign-in attempts from one address are counted.') },
    { path: 'auth.max_attempts', section: 'auth', title: $gettext('Max Attempts'), description: $gettext('Failed attempts allowed inside the window before the address is banned for a while.') },
    { path: 'auth.banned_ips', section: 'auth', title: $gettext('Banned IPs'), description: $gettext('Addresses that are currently blocked from signing in.') },

    // Access tokens
    { path: 'access_tokens', section: 'access_tokens', title: $gettext('Access Tokens'), description: $gettext('Create scoped credentials for the Nginx UI CLI, automation, and MCP clients. Tokens are shown only once.') },

    // Cert
    { path: 'cert.email', section: 'cert', title: $gettext('Email'), description: $gettext('Contact address registered with the certificate authority.') },
    { path: 'cert.http_challenge_port', section: 'cert', title: $gettext('HTTP Challenge Port'), description: $gettext('Port that answers validation requests while a certificate is being issued.') },
    { path: 'cert.ca_dir', section: 'cert', title: $gettext('CADir'), description: $gettext('Service that issues certificates. Leave empty to use the default.') },
    { path: 'cert.renewal_interval', section: 'cert', title: $gettext('Certificate Renewal Threshold'), description: $gettext('Renew certificates when their remaining validity is less than or equal to this value.') },

    // Plugin
    { path: 'plugin.enabled', section: 'plugin', title: $gettext('Plugin System'), description: $gettext('Turning it off stops every plugin and hides the plugin pages.') },
    { path: 'plugin.dir', section: 'plugin', title: $gettext('Plugin Directory') },
    { path: 'plugin.default_sync_policy', section: 'plugin', title: $gettext('Default Sync Policy'), description: $gettext('Applied to newly installed plugins. Automatic keeps the plugin installed on the child nodes.') },
    { path: 'plugin.marketplace_enabled', section: 'plugin', title: $gettext('Enable Marketplace') },
    { path: 'plugin.marketplace_sources', section: 'plugin', title: $gettext('Sources'), description: $gettext('Managed on the marketplace page. Empty means the official catalog.') },
    { path: 'plugin.allow_community_plugins', section: 'plugin', title: $gettext('Allow Community Plugins'), description: $gettext('Community plugins are published by third parties and ask for a confirmation before they install.') },
    { path: 'plugin.auto_update', section: 'plugin', title: $gettext('Automatic Updates'), description: $gettext('Updates official and partner plugins on their own while the permissions they ask for stay the same.') },
    { path: 'plugin.allow_uploads', section: 'plugin', title: $gettext('Allow Uploads'), description: $gettext('Allows installing packages uploaded from the browser or the command line.') },
    { path: 'plugin.trusted_public_keys', section: 'plugin', title: $gettext('Trusted Publishers'), description: $gettext('Plugins from these publishers install as community plugins.') },
    { path: 'plugin.memory_limit_mb', section: 'plugin', title: $gettext('Memory Limit'), description: $gettext('Maximum memory each plugin can use.') },
    { path: 'plugin.cpu_percent', section: 'plugin', title: $gettext('CPU Limit'), description: $gettext('Share of one CPU core each plugin can use.') },
    { path: 'plugin.allow_insecure_download_url', section: 'plugin', title: $gettext('Allow Insecure Download URLs'), description: $gettext('Accepts plain http catalog and download addresses. Only for a private catalog on a trusted network.') },
    { path: 'plugin.developer_mode', section: 'plugin', title: $gettext('Developer Mode'), description: $gettext('Allows installing unsigned plugins. Only turn this on while developing a plugin.') },

    // Nginx
    { path: 'nginx.stub_status_port', section: 'nginx', title: $gettext('Stub Status Port'), description: $gettext('Local port used to read Nginx connection statistics.') },
    { path: 'nginx.maintenance_host', section: 'nginx', title: $gettext('Maintenance host'), description: $gettext('Optional HTTP or HTTPS origin that serves the maintenance page. Leave empty to use this Nginx UI instance.') },
    { path: 'nginx.maintenance_bypass_ip', section: 'nginx', title: $gettext('Maintenance bypass IP'), description: $gettext('Requests from this IPv4 or IPv6 address continue to use the original site while maintenance mode is active.') },
    { path: 'nginx.maintenance_template', section: 'nginx', title: $gettext('Maintenance template (filename only)'), description: $gettext('The file named <site name>.<filename> is used first; if it does not exist, the generic <filename> is used; if neither exists, the built-in Nginx UI maintenance page is used.') },
    { path: 'nginx.host_mode', section: 'nginx', title: $gettext('Nginx Control Mode'), description: $gettext('How Nginx UI reaches the Nginx it manages.') },
    { path: 'nginx.access_log_path', section: 'nginx', title: $gettext('Nginx Access Log Path') },
    { path: 'nginx.error_log_path', section: 'nginx', title: $gettext('Nginx Error Log Path') },
    { path: 'nginx.config_dir', section: 'nginx', title: $gettext('Nginx Configurations Directory') },
    { path: 'nginx.config_path', section: 'nginx', title: $gettext('Nginx Configuration Path') },
    { path: 'nginx.log_dir_white_list', section: 'nginx', title: $gettext('Nginx Log Directory Whitelist') },
    { path: 'nginx.pid_path', section: 'nginx', title: $gettext('Nginx PID Path') },
    { path: 'nginx.test_config_cmd', section: 'nginx', title: $gettext('Nginx Test Config Command') },
    { path: 'nginx.reload_cmd', section: 'nginx', title: $gettext('Nginx Reload Command') },
    { path: 'nginx.restart_cmd', section: 'nginx', title: $gettext('Nginx Restart Command') },

    // LLM
    { path: 'openai.provider', section: 'openai', title: $gettext('Provider') },
    { path: 'openai.base_url', section: 'openai', title: $gettext('API Base Url') },
    { path: 'openai.token', section: 'openai', title: $gettext('API Token') },
    { path: 'openai.proxy', section: 'openai', title: $gettext('API Proxy') },
    { path: 'openai.api_type', section: 'openai', title: $gettext('API Type') },
    { path: 'openai.chat_models', section: 'openai', title: $gettext('Assistant models'), description: $gettext('The models to choose from in the assistant.') },
    { path: 'openai.model', section: 'openai', title: $gettext('Default model'), description: $gettext('Used for new chats, chat titles and, unless set below, code completion.') },
    { path: 'openai.thinking', section: 'openai', title: $gettext('Thinking'), description: $gettext('How each model switches its thinking levels. Pick the convention of your provider, or edit the request fields.') },
    { path: 'openai.enable_code_completion', section: 'openai', title: $gettext('Enable Code Completion') },
    { path: 'openai.code_completion_model', section: 'openai', title: $gettext('Code Completion Model'), description: $gettext('The model used for code completion, if not set, the chat model will be used.') },

    // Health check
    { path: 'site_check.enabled', section: 'health_check', title: $gettext('Enable site health checks'), panel: $gettext('Sites') },
    { path: 'site_check.concurrency', section: 'health_check', title: $gettext('Concurrency'), description: $gettext('Sites checked at the same time.'), panel: $gettext('Sites') },
    { path: 'site_check.interval_seconds', section: 'health_check', title: $gettext('Interval'), description: $gettext('Time between two checks of the same site.'), panel: $gettext('Sites') },
    { path: 'upstream_check.enabled', section: 'health_check', title: $gettext('Enable upstream health checks'), panel: $gettext('Proxy Targets') },
    { path: 'upstream_check.interval_seconds', section: 'health_check', title: $gettext('Interval'), description: $gettext('Time between two checks of the same proxy target.'), panel: $gettext('Proxy Targets') },

    // External notify
    { path: 'external_notify', section: 'external_notify', title: $gettext('External Notify'), description: $gettext('Channels that receive notifications from Nginx UI.') },

    // Terminal
    { path: 'terminal.start_cmd', section: 'terminal', title: $gettext('Terminal Start Command') },

    // Logrotate
    { path: 'logrotate.enabled', section: 'logrotate', title: $gettext('Enable Logrotate'), description: $gettext('Runs the rotation command on a schedule from Nginx UI. Mainly needed inside a Docker container.') },
    { path: 'logrotate.cmd', section: 'logrotate', title: $gettext('Command') },
    { path: 'logrotate.interval', section: 'logrotate', title: $gettext('Interval'), description: $gettext('Minutes between two runs.') },

    // GeoLite
    { path: 'nginx_log.geolite_database', section: 'geolite', title: $gettext('GeoLite2 Database'), description: $gettext('The GeoLite2 database provides geographic information for IP addresses. This is used for offline geographic analysis in log analytics.') },
    { path: 'nginx_log.index_custom_mmdb', section: 'geolite', title: $gettext('Custom MMDB'), description: $gettext('A custom database replaces the GeoLite2 download.') },
    { path: 'nginx_log.geo_map_path', section: 'geolite', title: $gettext('Map Boundary Directory'), description: $gettext('Keep files with names like 100000_full.json in this directory. Only the world map is provided by default.') },
  ]
}

// Label used in the save bar and in the search results.
export function settingEntryLabel(entry: SettingCatalogEntry) {
  return entry.panel ? `${entry.panel} · ${entry.title}` : entry.title
}

// Finds the entry for a changed path, falling back to the closest parent.
export function findSettingEntry(entries: SettingCatalogEntry[], path: string) {
  const segments = path.split('.')
  while (segments.length > 0) {
    const candidate = segments.join('.')
    const entry = entries.find(item => item.path === candidate)
    if (entry)
      return entry
    segments.pop()
  }
  return undefined
}
