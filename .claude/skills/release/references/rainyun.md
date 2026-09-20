# RainYun Cloud App Publication

Use this phase only after the GitHub Release is public and the user has explicitly authorized RainYun publication. RainYun is downstream distribution, not a prerequisite for creating the GitHub Release.

## Release Gates

1. Re-read the official GitHub Release and tag instead of relying on local or earlier results. Require a non-draft, non-prerelease release whose tag resolves to the intended release commit.
2. Require the relevant release workflows and artifacts to be complete. Verify that `uozi/nginx-ui:<version>` exists as a multi-platform OCI image and contains the architecture used by the selected RainYun region.
3. Inspect the current RainYun template schema and store before editing. Reuse the existing NGINX UI template (currently template `8035`) instead of creating a duplicate, and check whether the same version already exists.

## Template Contract

Keep the RainYun version aligned with the official container deployment:

- Use one container with `uozi/nginx-ui:<version>`; never use `latest`.
- Set these fixed environment variables:
  - `TZ=Asia/Shanghai`
  - `NGINX_UI_NODE_SKIP_INSTALLATION=true`
  - `NGINX_UI_IGNORE_DOCKER_SOCKET=true`
- Define these required user options:
  - `NGINX_UI_PREDEFINED_USER_NAME`, defaulting to `admin`.
  - `NGINX_UI_PREDEFINED_USER_PASSWORD`, generated randomly with the `NginxUI` prefix and RainYun's password-complexity validation.
- Provide persistent writable volumes for:
  - `/etc/nginx-ui`
  - `/etc/nginx`
  - `/var/www`
  - `/var/log/nginx`
- Do not mount the Docker socket and do not use a read-only ConfigMap for these writable paths.
- Expose container TCP port `80` as the web-management service. RainYun shared-IP deployments normally receive a random external port; do not promise public ports `80` or `443`. Standard HTTP/HTTPS requires RainYun website proxying or an independent public IP and must be verified separately.
- Preserve the current minimum resource baseline unless current platform requirements or a real deployment prove it insufficient. The established baseline is `0.5` CPU and `512 MB` memory.

## External-Action Gates

RainYun changes are separate representational actions. Obtain explicit authorization before creating or changing a template/version, saving an external draft, uploading artwork, listing a version, or submitting the application for review unless the user's current request already authorizes that exact scope.

Before clicking the final install/deploy control, show the selected region, resources, networking mode and external port, trial terms, displayed estimate, and potential ongoing charge. Obtain action-time confirmation because the deployment may incur cost. Stop for user handoff if a CAPTCHA or other human verification appears; never bypass it.

## Publication Sequence

1. Update the existing template metadata and version using the contract above. Use the repository logo and localized product copy consistent with the existing template unless the user requests other material.
2. List the version and read back its listed state. Application-level submission requires at least one listed version.
3. Deploy the template through its public store/install flow. Do not treat a saved template or successful version listing as deployment proof.
4. Wait for the application to reach a healthy/running state. Verify the allocated endpoint and a meaningful NGINX UI response, preferring a health endpoint when available. Inspect deployment logs if startup fails.
5. Submit the application to the store only after the successful template deployment is visible to RainYun. Read back the resulting review/listing state; a success toast alone is insufficient.
6. For the first listing or when the promotion URL changes, add or verify this README badge:

   `[![Deploy on RainYun](https://rainyun-apps.cn-nb1.rains3.com/materials/deploy-on-rainyun-en.svg)](https://app.rainyun.com/apps/rca/store/8035/nginxui_)`

   Keep this repository edit scoped. Do not stage or commit unrelated work, and do not infer commit/push authority from RainYun publication authority.

## Completion Evidence

Report the release tag and image digest/architectures, RainYun template and version IDs, deployment region/resources/network endpoint, runtime verification, final store state, and README diff/commit state. If review is pending, report it as pending instead of claiming the app is publicly listed.
