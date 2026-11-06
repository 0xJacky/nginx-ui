#!/bin/sh
# Build the peer nodes for the demo cluster view.
#
# Instead of running a second container, the demo runs extra nginx-ui processes
# inside the same container and points the cluster rows at 127.0.0.1. That keeps
# node-to-node traffic on loopback, so it never crosses the Cloudflare Worker
# boundary, never counts as an in-flight request (which would stop the container
# from ever sleeping), and needs no outbound interception.
#
# Two things must differ per instance:
#   1. the config directory, because the SQLite file is resolved as
#      dir(configPath)/<DatabaseSettings.Name>.db
#   2. NGINX_UI_WORKING_DIR, because risefront derives its handover socket path
#      from it (see prefix_dialer.go NewPrefixDialer). Sharing it makes the
#      second process attach to the first as a hot-reload child and give up its
#      own TCP port.
set -eu

: "${PEER_COUNT:=2}"
: "${BASE_CONFIG:=/etc/nginx-ui/app.ini}"
: "${BASE_DB:=/etc/nginx-ui/database.db}"

# Fixed instance IDs so the seeded node rows can reference them and so nothing
# rewrites app.ini on first boot.
peer_instance_id() {
    case "$1" in
        2) echo "1b7f4a52-9c3d-4e18-9a71-6d0f2c845b93" ;;
        3) echo "2c8e5b63-ad4e-4f29-8b62-7e1a3d956c04" ;;
        *) echo "00000000-0000-4000-8000-00000000000$1" ;;
    esac
}

n=2
while [ "$n" -le $((PEER_COUNT + 1)) ]; do
    # primary node holds 9000, so peer n takes 9000 + (n - 1)
    port=$((8999 + n))
    conf_dir="/etc/nginx-ui-node${n}"
    run_dir="/var/run/nginx-ui-node${n}"

    mkdir -p "$conf_dir" "$run_dir"

    # Peers exist to populate the cluster view. They share the single nginx in
    # this container, so leave the site prober and the log indexer to the
    # primary node rather than paying for them three times.
    sed -e "s|^Port .*|Port    = ${port}|" \
        -e "s|^Name .*|Name             = demo-node-${n}|" \
        -e "s|^InstanceID .*|InstanceID       = $(peer_instance_id "$n")|" \
        -e "s|^IndexingEnabled .*|IndexingEnabled = false|" \
        "$BASE_CONFIG" > "${conf_dir}/app.ini"
    sed -i "s|^Enabled         = true|Enabled         = false|" "${conf_dir}/app.ini"

    cp "$BASE_DB" "${conf_dir}/database.db"

    # resources/demo/entrypoint.sh discovers peers by looking for these config
    # files, so there is nothing else to register.
    echo "prepared demo peer node ${n} on port ${port} (${conf_dir})"
    n=$((n + 1))
done
