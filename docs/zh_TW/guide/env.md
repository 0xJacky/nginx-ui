# 環境變量
適用於 v2.0.0-beta.23 及以上版本

## Server

| Configuration Setting         | Environment Variable                  |
| ----------------------------- | ------------------------------------- |
| HttpPort                      | NGINX_UI_SERVER_HTTP_PORT             |
| RunMode                       | NGINX_UI_SERVER_RUN_MODE              |
| JwtSecret                     | NGINX_UI_SERVER_JWT_SECRET            |
| HTTPChallengePort             | NGINX_UI_SERVER_HTTP_CHALLENGE_PORT   |
| StartCmd                      | NGINX_UI_SERVER_START_CMD             |
| Database                      | NGINX_UI_SERVER_DATABASE              |
| CADir                         | NGINX_UI_SERVER_CA_DIR                |
| GithubProxy                   | NGINX_UI_SERVER_GITHUB_PROXY          |
| NodeSecret                    | NGINX_UI_SERVER_NODE_SECRET           |
| Demo                          | NGINX_UI_SERVER_DEMO                  |
| PageSize                      | NGINX_UI_SERVER_PAGE_SIZE             |
| HttpHost                      | NGINX_UI_SERVER_HTTP_HOST             |
| CertRenewalInterval           | NGINX_UI_SERVER_CERT_RENEWAL_INTERVAL |
| RecursiveNameservers          | NGINX_UI_SERVER_RECURSIVE_NAMESERVERS |
| SkipInstallation              | NGINX_UI_SERVER_SKIP_INSTALLATION     |
| Name                          | NGINX_UI_SERVER_NAME                  |

## Nginx

| Configuration Setting         | Environment Variable                  |
| ----------------------------- | ------------------------------------- |
| AccessLogPath                 | NGINX_UI_NGINX_ACCESS_LOG_PATH        |
| ErrorLogPath                  | NGINX_UI_NGINX_ERROR_LOG_PATH         |
| ConfigDir                     | NGINX_UI_NGINX_CONFIG_DIR             |
| PIDPath                       | NGINX_UI_NGINX_PID_PATH               |
| TestConfigCmd                 | NGINX_UI_NGINX_TEST_CONFIG_CMD        |
| ReloadCmd                     | NGINX_UI_NGINX_RELOAD_CMD             |
| RestartCmd                    | NGINX_UI_NGINX_RESTART_CMD            |

## OpenAI

| Configuration Setting         | Environment Variable                  |
| ----------------------------- | ------------------------------------- |
| Model                         | NGINX_UI_OPENAI_MODEL                 |
| BaseUrl                       | NGINX_UI_OPENAI_BASE_URL              |
| Proxy                         | NGINX_UI_OPENAI_PROXY                 |
| Token                         | NGINX_UI_OPENAI_TOKEN                 |

## Casdoor

| Configuration Setting         | Environment Variable                  |
| ----------------------------- | ------------------------------------- |
| Endpoint                      | NGINX_UI_CASDOOR_ENDPOINT             |
| ClientId                      | NGINX_UI_CASDOOR_CLIENT_ID            |
| ClientSecret                  | NGINX_UI_CASDOOR_CLIENT_SECRET        |
| Certificate                   | NGINX_UI_CASDOOR_CERTIFICATE          |
| Organization                  | NGINX_UI_CASDOOR_ORGANIZATION         |
| Application                   | NGINX_UI_CASDOOR_APPLICATION          |
| RedirectUri                   | NGINX_UI_CASDOOR_REDIRECT_URI         |

## Logrotate

| Configuration Setting         | Environment Variable                  |
| ----------------------------- | ------------------------------------- |
| Enabled                       | NGINX_UI_LOGROTATE_ENABLED            |
| CMD                           | NGINX_UI_LOGROTATE_CMD                |
| Interval                      | NGINX_UI_LOGROTATE_INTERVAL           |

## Auth

| Configuration Setting | Environment Variable        |
|-----------------------|-----------------------------|
| IPWhiteList           | NGINX_UI_AUTH_IPWhiteList   |

## 預定義使用者

在跳過安裝模式下，您可以設置以下環境變量以創建預定義使用者：

- NGINX_UI_PREDEFINED_USER_NAME
- NGINX_UI_PREDEFINED_USER_PASSWORD
