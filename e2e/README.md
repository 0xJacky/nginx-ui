# Public demo end-to-end tests

This directory is an isolated Playwright package for the Cloudflare Containers public demo. It does not participate in the `app/` Bun workspace or change the frontend build.

## Prerequisites

- Bun
- Docker with Linux/amd64 emulation available
- The repository frontend already built in `app/dist`, or network access so `cloudflare/build-binary.sh` can build it

Install the isolated dependencies and Chromium once:

```sh
cd e2e
bun install
bun run install:browser
```

## Start the local demo target

From the repository root:

```sh
sh cloudflare/build-binary.sh
docker build --platform linux/amd64 -f demo.Dockerfile -t nginxui-demo-e2e:local \
  --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64 .
docker run -d --name nginxui-e2e --platform linux/amd64 --cap-drop=ALL \
  -p 18110:8080 nginxui-demo-e2e:local
until curl --fail --silent http://127.0.0.1:18110/healthz >/dev/null; do sleep 2; done
```

`--cap-drop=ALL` is required because it reproduces the Cloudflare Containers capability set.

## Run the suite

The complete suite runs with one command from the repository root:

```sh
bun run --cwd e2e test
```

The default target is `http://127.0.0.1:18110` with `admin` / `admin`. Override it when needed:

```sh
E2E_BASE_URL=https://demo.example.test E2E_USERNAME=admin E2E_PASSWORD=admin \
  bun run --cwd e2e test
```

Failure traces, videos, and screenshots are retained under `e2e/test-results/`. The HTML report is written to `e2e/playwright-report/`.

Stop and remove the local target when finished:

```sh
docker rm -f nginxui-e2e
```
