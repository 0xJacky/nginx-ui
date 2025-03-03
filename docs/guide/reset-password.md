# Reset Initial User Password

The `reset-password` command allows you to reset the initial administrator account's password to a randomly generated 12-character password that includes uppercase letters, lowercase letters, numbers, and special symbols.

## Usage

To reset the initial user's password, run:

```bash
nginx-ui reset-password --config=/path/to/app.ini
```

The command will:
1. Generate a secure random password (12 characters)
2. Reset the password for the initial user account (user ID 1)
3. Output the new password in the application logs

## Parameters

- `--config`: (Required) Path to the Nginx UI configuration file

## Example

```bash
# Reset the password using the default config file location
nginx-ui reset-password --config=/path/to/app.ini

# The output will include the generated password
2025-03-03 03:24:41     INFO    user/reset_password.go:52       confPath: ../app.ini
2025-03-03 03:24:41     INFO    user/reset_password.go:59       dbPath: ../database.db
2025-03-03 03:24:41     INFO    user/reset_password.go:92       User: root, Password: X&K^(X0m(E&&
```

## Configuration File Location

- If you installed Nginx UI using the Linux one-click installation script, the configuration file is located at:
  ```
  /usr/local/etc/nginx-ui/app.ini
  ```

  You can directly use the following command:
  ```bash
  nginx-ui reset-password --config /usr/local/etc/nginx-ui/app.ini
  ```

## Docker Usage

If you're running Nginx UI in a Docker container, you need to use the `docker exec` command:

```bash
docker exec -it <nginx-ui-container> nginx-ui reset-password --config=/etc/nginx-ui/app.ini
```

Replace `<nginx-ui-container>` with your actual container name or ID.

## Notes

- This command is useful if you've forgotten the initial administrator password
- The new password will be displayed in the logs, so be sure to copy it immediately
- You must have access to the server's command line to use this feature
- The database file must exist for this command to work 