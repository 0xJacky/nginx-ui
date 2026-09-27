# shellcheck shell=sh
# Shared defaults for the ACME E2E scripts. Source it; do not execute it.
# The sourcing script's $0 locates the repository. Every value can be
# overridden from the environment. lib/env.ts mirrors these
# defaults for the Playwright side, so keep the two in sync.

ACME_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
REPO_ROOT="$(CDPATH='' cd -- "$ACME_DIR/../.." && pwd)"

: "${E2E_ACME_PROJECT:=nginxui-acme-e2e-suite}"
: "${E2E_ACME_UI_PORT:=18181}"
: "${E2E_ACME_CHALLTESTSRV_PORT:=18056}"
: "${E2E_ACME_SUBNET_PREFIX:=10.31.0}"
: "${NGINX_UI_IMAGE:=nginx-ui-acme-e2e:local}"

if [ -z "${E2E_ACME_DATA_DIR:-}" ]; then
    _acme_tmp="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
    E2E_ACME_DATA_DIR="${_acme_tmp%/}/$E2E_ACME_PROJECT"
    unset _acme_tmp
fi

export ACME_DIR REPO_ROOT E2E_ACME_PROJECT E2E_ACME_UI_PORT E2E_ACME_CHALLTESTSRV_PORT \
    E2E_ACME_SUBNET_PREFIX NGINX_UI_IMAGE E2E_ACME_DATA_DIR

acme_compose() {
    docker compose -p "$E2E_ACME_PROJECT" -f "$ACME_DIR/compose.yml" "$@"
}

acme_log() {
    echo "[acme-e2e] $*" >&2
}
