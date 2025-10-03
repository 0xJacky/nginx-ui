# 環境變數

適用於 v2.0.0-beta.37 及以上版本。

## App

| 設定        | 環境變數                    |
|-----------|-------------------------|
| PageSize  | NGINX_UI_APP_PAGE_SIZE  |
| JwtSecret | NGINX_UI_APP_JWT_SECRET |

## Server

| 設定        | 環境變數                         |
|-----------|------------------------------|
| Host      | NGINX_UI_SERVER_HOST         |
| Port      | NGINX_UI_SERVER_PORT         |
| RunMode   | NGINX_UI_SERVER_RUN_MODE     |
| EnableHTTPS | NGINX_UI_SERVER_ENABLE_HTTPS |
| EnableH2  | NGINX_UI_SERVER_ENABLE_H2    |
| EnableH3  | NGINX_UI_SERVER_ENABLE_H3    |

## Database

| 設定   | 環境變數             |
|------|------------------|
| Name | NGINX_UI_DB_NAME |

## Auth

| 設定                  | 環境變數                                |
|---------------------|-------------------------------------|
| IPWhiteList         | NGINX_UI_AUTH_IP_WHITE_LIST         |
| BanThresholdMinutes | NGINX_UI_AUTH_BAN_THRESHOLD_MINUTES |
| MaxAttempts         | NGINX_UI_AUTH_MAX_ATTEMPTS          |

## Casdoor

| 設定              | 環境變數                              |
|-----------------|-----------------------------------|
| Endpoint        | NGINX_UI_CASDOOR_ENDPOINT         |
| ClientId        | NGINX_UI_CASDOOR_CLIENT_ID        |
| ClientSecret    | NGINX_UI_CASDOOR_CLIENT_SECRET    |
| CertificatePath | NGINX_UI_CASDOOR_CERTIFICATE_PATH |
| Organization    | NGINX_UI_CASDOOR_ORGANIZATION     |
| Application     | NGINX_UI_CASDOOR_APPLICATION      |
| RedirectUri     | NGINX_UI_CASDOOR_REDIRECT_URI     |

## Cert

| 設定                   | 環境變數                                |
|----------------------|-------------------------------------|
| Email                | NGINX_UI_CERT_EMAIL                 |
| CADir                | NGINX_UI_CERT_CA_DIR                |
| RenewalInterval      | NGINX_UI_CERT_RENEWAL_INTERVAL      |
| RecursiveNameservers | NGINX_UI_CERT_RECURSIVE_NAMESERVERS |
| HTTPChallengePort    | NGINX_UI_CERT_HTTP_CHALLENGE_PORT   |

## Cluster

| 設定   | 環境變數                  |
|------|-----------------------|
| Node | NGINX_UI_CLUSTER_NODE |

## Crypto

| 設定     | 環境變數                   |
|--------|------------------------|
| Secret | NGINX_UI_CRYPTO_SECRET |

## Http

| 設定                 | 環境變數                               |
|--------------------|------------------------------------|
| GithubProxy        | NGINX_UI_HTTP_GITHUB_PROXY         |
| InsecureSkipVerify | NGINX_UI_HTTP_INSECURE_SKIP_VERIFY |

## Logrotate

| 設定       | 環境變數                        |
|----------|-----------------------------|
| Enabled  | NGINX_UI_LOGROTATE_ENABLED  |
| CMD      | NGINX_UI_LOGROTATE_CMD      |
| Interval | NGINX_UI_LOGROTATE_INTERVAL |

## Nginx

| 設定              | 環境變數                              |
|-----------------|-----------------------------------|
| AccessLogPath   | NGINX_UI_NGINX_ACCESS_LOG_PATH    |
| ErrorLogPath    | NGINX_UI_NGINX_ERROR_LOG_PATH     |
| ConfigDir       | NGINX_UI_NGINX_CONFIG_DIR         |
| PIDPath         | NGINX_UI_NGINX_PID_PATH           |
| SbinPath        | NGINX_UI_NGINX_SBIN_PATH          |
| TestConfigCmd   | NGINX_UI_NGINX_TEST_CONFIG_CMD    |
| ReloadCmd       | NGINX_UI_NGINX_RELOAD_CMD         |
| RestartCmd      | NGINX_UI_NGINX_RESTART_CMD        |
| LogDirWhiteList | NGINX_UI_NGINX_LOG_DIR_WHITE_LIST |
| StubStatusPort  | NGINX_UI_NGINX_STUB_STATUS_PORT   |
| ContainerName   | NGINX_UI_NGINX_CONTAINER_NAME     |

## Nginx Log

| 設定                     | 環境變數                                       |
|-------------------------|-----------------------------------------------|
| IndexingEnabled | NGINX_UI_NGINX_LOG_INDEXING_ENABLED |
| IndexPath               | NGINX_UI_NGINX_LOG_INDEX_PATH                |

## Node

| 設定               | 環境變數                            |
|------------------|---------------------------------|
| Name             | NGINX_UI_NODE_NAME              |
| Secret           | NGINX_UI_NODE_SECRET            |
| SkipInstallation | NGINX_UI_NODE_SKIP_INSTALLATION |

## OpenAI

| 設定      | 環境變數                     |
|---------|--------------------------|
| Model   | NGINX_UI_OPENAI_MODEL    |
| BaseUrl | NGINX_UI_OPENAI_BASE_URL |
| Proxy   | NGINX_UI_OPENAI_PROXY    |
| Token   | NGINX_UI_OPENAI_TOKEN    |

## Terminal

| 設定       | 環境變數                        |
|----------|-----------------------------|
| StartCmd | NGINX_UI_TERMINAL_START_CMD |

## Webauthn

| 設定            | 環境變數                              |
|---------------|-----------------------------------|
| RPDisplayName | NGINX_UI_WEBAUTHN_RP_DISPLAY_NAME |
| RPID          | NGINX_UI_WEBAUTHN_RPID            |
| RPOrigins     | NGINX_UI_WEBAUTHN_RP_ORIGINS      |

## 預定義使用者

在跳過安裝模式下，您可以設定以下環境變數以建立預定義使用者：

- NGINX_UI_PREDEFINED_USER_NAME
- NGINX_UI_PREDEFINED_USER_PASSWORD
