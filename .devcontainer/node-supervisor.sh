#!/bin/bash

# Configurable variables
SOURCE_FILE=/workspaces/nginx-ui/tmp/main
TARGET_PATH=/usr/local/bin/nginx-ui
CONFIG_FILE=/etc/nginx-ui/app.ini

# init nginx
./.devcontainer/init-nginx.sh

LOG_PREFIX="[Supervisor]"

# Debug initial state
echo "$LOG_PREFIX Starting supervisor with:"
echo "$LOG_PREFIX SOURCE_FILE: $SOURCE_FILE"
echo "$LOG_PREFIX TARGET_PATH: $TARGET_PATH"
echo "$LOG_PREFIX CONFIG_FILE: $CONFIG_FILE"

# Wait for initial file creation
while [[ ! -f "$SOURCE_FILE" ]]; do
    echo "$LOG_PREFIX Waiting for $SOURCE_FILE to be created..."
    sleep 1
done

# Initial copy and start
echo "$LOG_PREFIX Initial file detected, starting service..."
cp -fv "$SOURCE_FILE" "$TARGET_PATH"
chmod +x "$TARGET_PATH"
pkill -x nginx-ui || echo "$LOG_PREFIX No existing process to kill"
nohup "$TARGET_PATH" -config "$CONFIG_FILE" > /proc/1/fd/1 2>&1 &

# Use proper field separation for inotify output
inotifywait -m -e close_write,moved_to,create,delete \
    --format "%T|%w%f|%e" \
    --timefmt "%F-%H:%M:%S" \
    "$(dirname "$SOURCE_FILE")" |
while IFS='|' read -r TIME FILE EVENT; do
    echo "$LOG_PREFIX [${TIME}] Event: ${EVENT} - ${FILE}"
    
    # Handle atomic save operations
    if [[ "$FILE" =~ .*-tmp-umask$ ]] || [[ "$EVENT" == "DELETE" ]]; then
        echo "$LOG_PREFIX Detected build intermediate file, checking main..."
        sleep 0.3  # Allow atomic replace completion
        
        if [[ -f "$SOURCE_FILE" ]]; then
            echo "$LOG_PREFIX Valid main file detected after build"
            FILE="$SOURCE_FILE"
        else
            echo "$LOG_PREFIX Main file missing after build operation"
            continue
        fi
    fi

    if [[ "$FILE" == "$SOURCE_FILE" ]]; then
        # Stability checks
        echo "$LOG_PREFIX File metadata:"
        ls -l "$FILE"
        file "$FILE"
        
        # Wait for file stability with retries
        retries=5
        while ((retries-- > 0)); do
            if file "$FILE" | grep -q "executable"; then
                break
            fi
            echo "$LOG_PREFIX Waiting for valid executable (${retries} retries left)..."
            sleep 1
        done

        if ((retries <= 0)); then
            echo "$LOG_PREFIX ERROR: File validation failed after 5 retries"
            continue
        fi

        # Copy and restart service
        echo "$LOG_PREFIX Updating service..."
        cp -fv "$FILE" "$TARGET_PATH"
        chmod +x "$TARGET_PATH"
        
        echo "$LOG_PREFIX Killing existing process..."
        pkill -x nginx-ui || echo "$LOG_PREFIX No process to kill"
        
        echo "$LOG_PREFIX Starting new process..."
        nohup "$TARGET_PATH" -config "$CONFIG_FILE" > /proc/1/fd/1 2>&1 &
        echo "$LOG_PREFIX Restart complete. New PID: $(pgrep nginx-ui)"
    fi
done
