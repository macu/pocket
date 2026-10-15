# Pocket

## Install Docker and Docker Compose

This command runs on Mac.

```bash
brew install --cask docker
```

## Create .env file (local development)

```
DB_USER=dbuser
DB_PASS=dbpass
DB_NAME=pocket
PORT=8080
DOMAIN=localhost:2026
MAINTENANCE_MODE=false
VERSION_STAMP=
RECAPTCHA_KEY=
GOOGLE_CLOUD_PROJECT=
MAILJET_API_KEY=
MAILJET_SECRET=
MAILJET_FROM_EMAIL=
OPENAI_SECRET_KEY=
```

## Links in emails

Links in emails are built from `DOMAIN` (host with optional port, e.g.
`localhost:2026`), using `http` locally and `https` on App Engine. Include a
scheme (e.g. `https://dev.example`) to override this.

## AI moderation

New topics, posts, and post edits are checked with the OpenAI moderation API
(`omni-moderation-latest`). Set `OPENAI_SECRET_KEY` to enable it; when unset,
moderation is skipped locally and requests fail in App Engine.

## reCAPTCHA and local testing

Set `RECAPTCHA_KEY` to a score-based Enterprise site key and
`GOOGLE_CLOUD_PROJECT` to its project. Allow `localhost` on the key and enable
the reCAPTCHA Enterprise API.

For local Docker development, install `gcloud` and configure Application Default
Credentials (ADC):

```bash
brew install --cask gcloud-cli
gcloud auth application-default login
gcloud auth application-default set-quota-project YOUR_GOOGLE_CLOUD_PROJECT
```

Compose mounts `~/.config/gcloud/application_default_credentials.json` read-only
and sets `GOOGLE_APPLICATION_CREDENTIALS`. Recreate the container after environment
changes, then reload the browser:

```bash
docker compose up -d --no-deps web
```

Authenticated POSTs require a matching token action and score of at least `0.5`,
including locally. Failed responses include `score` and `reason`; check the browser
Network panel and `docker compose logs --tail 100 web` for diagnostics.
Production uses its attached service account, not local ADC credentials.

## Start up containers

```bash
cd pocket
sh ./bin/restart-all.sh
```

## Init DB and access postgres interactive shell

Run `sql/init.pgsql` by passing contents into psql.

```bash
sh ./bin/psql-execute-script.sh < sql/init.pgsql
sh ./bin/psql-shell.sh
```

For an existing database, apply any SQL scripts in `sql/migrations/` before
deploying the updated application.

## Rebuild web app

The app will re-compile automatically when any Go source files change.

Run the following command to force the web container to restart.

```bash
sh ./bin/restart-web.sh
```

Run the following command to re-compile the frontend web bundle.

```bash
sh rebuild.sh
```

## Test in browser

http://localhost:2026/
