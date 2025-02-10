# Environment Variables

Applicable for version v2.0.0-beta.37 and above.

## App
| Configuration Setting | Environment Variable    |
|-----------------------|-------------------------|
| PageSize              | NGINX_UI_APP_PAGE_SIZE  |
| JwtSecret             | NGINX_UI_APP_JWT_SECRET |


## Server
| Configuration Setting | Environment Variable                  |
|-----------------------|---------------------------------------|
| Host                  | NGINX_UI_SERVER_HOST                  |
| Port                  | NGINX_UI_SERVER_PORT                  |
| RunMode               | NGINX_UI_SERVER_RUN_MODE              |

## Database
| Configuration Setting | Environment Variable |
|-----------------------|----------------------|
| Name                  | NGINX_UI_DB_NAME     |

## Auth
| Configuration Setting | Environment Variable                |
|-----------------------|-------------------------------------|
| IPWhiteList           | NGINX_UI_AUTH_IP_WHITE_LIST         |
| BanThresholdMinutes   | NGINX_UI_AUTH_BAN_THRESHOLD_MINUTES |
| MaxAttempts           | NGINX_UI_AUTH_MAX_ATTEMPTS          |

## Casdoor
| Configuration Setting | Environment Variable              |
|-----------------------|-----------------------------------|
| Endpoint              | NGINX_UI_CASDOOR_ENDPOINT         |
| ClientId              | NGINX_UI_CASDOOR_CLIENT_ID        |
| ClientSecret          | NGINX_UI_CASDOOR_CLIENT_SECRET    |
| CertificatePath       | NGINX_UI_CASDOOR_CERTIFICATE_PATH |
| Organization          | NGINX_UI_CASDOOR_ORGANIZATION     |
| Application           | NGINX_UI_CASDOOR_APPLICATION      |
| RedirectUri           | NGINX_UI_CASDOOR_REDIRECT_URI     |

## Cert
| Configuration Setting | Environment Variable                |
|-----------------------|-------------------------------------|
| Email                 | NGINX_UI_CERT_EMAIL                 |
| CADir                 | NGINX_UI_CERT_CA_DIR                |
| RenewalInterval       | NGINX_UI_CERT_RENEWAL_INTERVAL      |
| RecursiveNameservers  | NGINX_UI_CERT_RECURSIVE_NAMESERVERS |
| HTTPChallengePort     | NGINX_UI_CERT_HTTP_CHALLENGE_PORT   |

## Cluster
| Configuration Setting | Environment Variable  |
|-----------------------|-----------------------|
| Node                  | NGINX_UI_CLUSTER_NODE |

## Crypto
| Configuration Setting | Environment Variable    |
|-----------------------|-------------------------|
| Secret                | NGINX_UI_CRYPTO_SECRET  |

## Http
| Configuration Setting | Environment Variable               |
|-----------------------|------------------------------------|
| GithubProxy           | NGINX_UI_HTTP_GITHUB_PROXY         |
| InsecureSkipVerify    | NGINX_UI_HTTP_INSECURE_SKIP_VERIFY |

## Logrotate
| Configuration Setting | Environment Variable        |
|-----------------------|-----------------------------|
| Enabled               | NGINX_UI_LOGROTATE_ENABLED  |
| CMD                   | NGINX_UI_LOGROTATE_CMD      |
| Interval              | NGINX_UI_LOGROTATE_INTERVAL |

## Nginx
| Configuration Setting | Environment Variable              |
|-----------------------|-----------------------------------|
| AccessLogPath         | NGINX_UI_NGINX_ACCESS_LOG_PATH    |
| ErrorLogPath          | NGINX_UI_NGINX_ERROR_LOG_PATH     |
| ConfigDir             | NGINX_UI_NGINX_CONFIG_DIR         |
| PIDPath               | NGINX_UI_NGINX_PID_PATH           |
| TestConfigCmd         | NGINX_UI_NGINX_TEST_CONFIG_CMD    |
| ReloadCmd             | NGINX_UI_NGINX_RELOAD_CMD         |
| RestartCmd            | NGINX_UI_NGINX_RESTART_CMD        |
| LogDirWhiteList       | NGINX_UI_NGINX_LOG_DIR_WHITE_LIST |

## Node
| Configuration Setting | Environment Variable            |
|-----------------------|---------------------------------|
| Name                  | NGINX_UI_NODE_NAME              |
| Secret                | NGINX_UI_NODE_SECRET            |
| SkipInstallation      | NGINX_UI_NODE_SKIP_INSTALLATION |

## OpenAI
| Configuration Setting | Environment Variable     |
|-----------------------|--------------------------|
| Model                 | NGINX_UI_OPENAI_MODEL    |
| BaseUrl               | NGINX_UI_OPENAI_BASE_URL |
| Proxy                 | NGINX_UI_OPENAI_PROXY    |
| Token                 | NGINX_UI_OPENAI_TOKEN    |

## Terminal
| Configuration Setting | Environment Variable                |
|-----------------------|-------------------------------------|
| StartCmd              | NGINX_UI_TERMINAL_START_CMD         |

## Webauthn

| Configuration Setting | Environment Variable              |
|-----------------------|-----------------------------------|
| RPDisplayName         | NGINX_UI_WEBAUTHN_RP_DISPLAY_NAME |
| RPID                  | NGINX_UI_WEBAUTHN_RPID            |
| RPOrigins             | NGINX_UI_WEBAUTHN_RP_ORIGINS      |

## Predefined User

In skip installation mode, you can set the following environment variables to create a predefined user:

- NGINX_UI_PREDEFINED_USER_NAME
- NGINX_UI_PREDEFINED_USER_PASSWORD
