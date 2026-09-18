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
MAINTENANCE_MODE=false
VERSION_STAMP=
RECAPTCHA_SITE_KEY=
RECAPTCHA_SECRET=
MAILJET_API_KEY=
MAILJET_SECRET=
```

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
