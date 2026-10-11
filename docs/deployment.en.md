# Docker deployment and tag releases

[Back to the project](../README.en.md) · [简体中文](deployment.md) / English

Images support **Linux x86_64 / linux/amd64**. Image deployment requires Docker Engine with Compose. An initializer generates database credentials, encryption keys and internal service certificates. Both browser-facing entry points use HTTP and can sit behind your existing reverse proxy.

## 1. Publish a tag

Commit the changes to your repository, then push a tag:

```bash
git push origin HEAD
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions publishes `ghcr.io/gucooing/apophis-teamseatwatch:v1.0.0` and a matching GitHub Release containing `compose.yaml`, `env.example` and deployment commands. The image version is the exact Git tag, including any `v` prefix. Forks automatically use their own lowercased owner/repository image path.

Tags must also be valid Docker tags: 1–128 ASCII letters, digits, underscores, dots or dashes; the first character cannot be a dot or dash. Tags containing slashes are rejected instead of being silently renamed. Publication uses the built-in `GITHUB_TOKEN`.

For anonymous pulls, set the package visibility to **Public** in GitHub Packages after its first publication. New container packages default to Private. Private packages can be pulled after `docker login ghcr.io` using a classic PAT with `read:packages`. See the [GitHub Container Registry documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry).

## 2. Deploy directly with Docker

The example uses an already published `v1.0.0`; replace that tag for another version. No source checkout is required:

```bash
mkdir -p teamseatwatch
cd teamseatwatch
curl -fL https://github.com/gucooing/Apophis-TeamSeatWatch/releases/download/v1.0.0/compose.yaml -o compose.yaml
curl -fL https://github.com/gucooing/Apophis-TeamSeatWatch/releases/download/v1.0.0/env.example -o .env
```

Edit `.env` to select the image and the admin origin used by your browser:

```dotenv
TSW_IMAGE=ghcr.io/gucooing/apophis-teamseatwatch:v1.0.0
COMPOSE_PROJECT_NAME=teamseatwatch
TSW_OWNER_ORIGINS=https://admin.example.com
TSW_BIND_ADDRESS=127.0.0.1
TSW_OWNER_PORT=18443
TSW_REDEEM_PORT=18444
```

Origins contain no `/owner/` path or trailing slash. For local HTTP access, use the example file's `http://localhost:18443,http://127.0.0.1:18443`. When changing the admin port, also update the port in its origin.

```bash
docker compose up -d
docker compose logs control
```

The first start creates the `owner` administrator and logs:

```json
{"msg":"owner_initialized","username":"owner","password":"the generated random password"}
```

Use that password to sign in. Credentials are generated and logged only when the database has no administrator. Restarts and upgrades reuse the existing account. Custom passwords have no minimum length and must be nonempty. Automatically generated passwords remain 32 random characters.

| Entry | Local address / reverse proxy upstream |
| --- | --- |
| Admin workspace | `http://127.0.0.1:18443/owner/` |
| Customer redemption | `http://127.0.0.1:18444/redeem/` |

Compose volumes persist the database, business data and secrets. Browser-facing HTTP requires no application-generated web certificate or browser CA import. The initializer configures private control/gateway mTLS automatically.

A source checkout can deploy the same image:

```bash
cp deploy/.env.example .env
# Edit the image tag and admin origin in .env.
docker compose --env-file .env -f deploy/compose.yaml up -d
docker compose --env-file .env -f deploy/compose.yaml logs control
```

## 3. Use your reverse proxy

Forward the admin domain to `http://127.0.0.1:18443` and the redemption domain to `http://127.0.0.1:18444`, preserving URL paths. Inside your existing Nginx virtual hosts:

```nginx
# Existing admin server
location / {
    proxy_pass http://127.0.0.1:18443;
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 300s;
    client_max_body_size 20m;
}

# Existing redemption server
location / {
    proxy_pass http://127.0.0.1:18444;
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 300s;
}
```

Your proxy manages the external domains and HTTPS certificates. HTTPS admin origins enable Secure cookies; HTTP-only origins support direct HTTP login. Origin and CSRF validation use the configured browser address.

For a reverse proxy on another host or in a separate container, configure `TSW_BIND_ADDRESS` and the upstream host address for that network. `TSW_BIND_ADDRESS=0.0.0.0` publishes the entry points on all host interfaces; the default binds to host loopback.

## 4. ChatGPT client settings

Web and Codex identities are configured separately. The default web UA follows the Chromium version installed in the image. The browser and HTTP requests use matching UA, language and timezone settings. The project-specific `Apophis-TeamSeatWatch/1` UA, fixed Windows Chrome sample, stale web build numbers and static observation markers have been removed.

| Variable | Default and purpose |
| --- | --- |
| `TSW_CLIENT_USER_AGENT` | Empty uses the installed Chromium UA; overrides apply to web HTTP requests and Chromium |
| `TSW_CLIENT_ACCEPT_LANGUAGE` | `en-US,en;q=0.9`; used by HTTP Accept-Language and browser locale |
| `TSW_CLIENT_TIMEZONE` | `UTC`; an IANA timezone such as `Asia/Taipei`, also used by workspace discovery |
| `TSW_CLIENT_WEB_VERSION` | Optional actual web version; empty omits Oai-Client-Version |
| `TSW_CLIENT_WEB_BUILD` | Optional actual build number; empty omits Oai-Client-Build-Number |
| `TSW_CLIENT_CODEX_USER_AGENT` | `codex_cli_rs/0.146.0`; must retain the Codex format and is independent of the web UA |

Configure values for your deployment in `.env`, then run `docker compose up -d` again. Custom web UAs should match the actual browser environment. Settings remain stable within a process; device and session IDs continue to come from stored account sessions.

Codex OAuth, token exchange, refresh and usage requests retain their existing Codex protocol and HTTP transport with an independent Codex UA. A web UA override does not convert them to browser requests. OAuth client ID, scope, originator and PKCE validation retain their protocol requirements.

These changes reduce repeated header and version samples. Go HTTP TLS behavior, egress IPs and access patterns remain observable; identification or upstream restrictions cannot be ruled out.

## 5. Maintenance

From the directory holding the downloaded Compose file:

```bash
docker compose ps
docker compose logs --tail 100 control gateway
docker compose stop
docker compose up -d
```

Reset a forgotten admin password to any nonempty value; existing sessions are revoked:

```bash
docker compose run --rm --no-deps -e TSW_OWNER_LOGIN=owner -e TSW_OWNER_PASSWORD='new-password' cli owner-reset
```

To upgrade, change `TSW_IMAGE` in `.env` to the new tag while keeping the deployment directory, `COMPOSE_PROJECT_NAME` and volumes:

```bash
docker compose stop control gateway
docker compose pull
docker compose up -d
```

Before migration, back up the database together with all corresponding secrets volumes. Restore the database and keys from the same backup.

## 6. Build from source

Source builds use the same Compose configuration and require no Python installer:

```bash
git clone https://github.com/gucooing/Apophis-TeamSeatWatch.git
cd Apophis-TeamSeatWatch
docker build --platform linux/amd64 -t teamseatwatch:local .
cp deploy/.env.example .env
# Set TSW_IMAGE=teamseatwatch:local and the admin origin in .env.
docker compose --env-file .env -f deploy/compose.yaml up -d
docker compose --env-file .env -f deploy/compose.yaml logs control
```

Initialization, HTTP entry points, generated admin credentials and client settings work the same way as image deployment. The old Python installer and HTTPS proxy setup have been removed.
