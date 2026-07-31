#!/usr/bin/env bash

set -euo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
templates_dir="${repository_dir}/templates"
expected_repository_url="https://github.com/nginxui/unraid"
expected_raw_root="https://raw.githubusercontent.com/nginxui/unraid/main"
expected_raw_base="${expected_raw_root}/templates"

command -v xmllint >/dev/null 2>&1 || {
  echo "xmllint is required" >&2
  exit 1
}

test -s "${repository_dir}/LICENSE"
test -s "${repository_dir}/README.md"

templates=("${templates_dir}"/*.xml)
test "${#templates[@]}" -eq 2

xmllint --noout "${repository_dir}/ca_profile.xml" "${repository_dir}/icon.svg" "${templates[@]}"

profile="$(xmllint --xpath 'string(/CommunityApplications/Profile)' "${repository_dir}/ca_profile.xml")"
test -n "${profile}"
test "$(xmllint --xpath 'string(/CommunityApplications/WebPage)' "${repository_dir}/ca_profile.xml")" = "${expected_repository_url}"
test "$(xmllint --xpath 'string(/CommunityApplications/Icon)' "${repository_dir}/ca_profile.xml")" = "${expected_raw_root}/icon.svg"

for template in "${templates[@]}"; do
  test "$(xmllint --xpath 'string(/Container/@version)' "${template}")" = "2"
  test "$(xmllint --xpath 'string(/Container/Repository)' "${template}")" = "uozi/nginx-ui:latest"
  test "$(xmllint --xpath 'string(/Container/Privileged)' "${template}")" = "false"
  test "$(xmllint --xpath 'string(/Container/Icon)' "${template}")" = "${expected_raw_root}/icon.svg"
  test "$(xmllint --xpath 'string(/Container/TemplateURL)' "${template}")" = "${expected_raw_base}/$(basename "${template}")"
  test "$(xmllint --xpath 'string(/Container/ReadMe)' "${template}")" = "${expected_raw_root}/README.md"
  test -n "$(xmllint --xpath 'string(/Container/Support)' "${template}")"
  test -z "$(xmllint --xpath 'string(/Container/Config[@Target="PUID"]/@Default)' "${template}")"
  test -z "$(xmllint --xpath 'string(/Container/Config[@Target="PGID"]/@Default)' "${template}")"
done

standalone="${templates_dir}/nginx-ui.xml"
swag="${templates_dir}/nginx-ui-swag.xml"

test "$(xmllint --xpath 'string(/Container/Config[@Target="NGINX_UI_IGNORE_DOCKER_SOCKET"]/@Default)' "${standalone}")" = "true"
test -z "$(xmllint --xpath 'string(/Container/Config[@Target="/var/run/docker.sock"]/@Default)' "${standalone}")"

test "$(xmllint --xpath 'string(/Container/Config[@Target="NGINX_UI_DISABLE_BUNDLED_NGINX"]/@Default)' "${swag}")" = "true"
test "$(xmllint --xpath 'string(/Container/Config[@Target="NGINX_UI_NGINX_CONTAINER_NAME"]/@Default)' "${swag}")" = "swag"
test "$(xmllint --xpath 'string(/Container/Config[@Target="/config"]/@Default)' "${swag}")" = "/mnt/user/appdata/swag"
test "$(xmllint --xpath 'string(/Container/Config[@Target="/var/run/docker.sock"]/@Default)' "${swag}")" = "/var/run/docker.sock"
test "$(xmllint --xpath 'string(/Container/Config[@Target="NGINX_UI_NGINX_CONFIG_PATH"]/@Default)' "${swag}")" = "/config/nginx/nginx.conf"
test "$(xmllint --xpath 'string(/Container/Config[@Target="NGINX_UI_NGINX_ACCESS_LOG_PATH"]/@Default)' "${swag}")" = "/config/log/nginx/access.log"

if grep -REn --include='*.xml' 'YOUR_|PUID|PGID' "${repository_dir}"; then
  echo "placeholder or unsupported PUID/PGID setting found" >&2
  exit 1
fi

echo "Unraid Community Applications templates are valid"
