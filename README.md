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
RECAPTCHA_KEY=
GOOGLE_CLOUD_PROJECT=
MAILJET_API_KEY=
MAILJET_SECRET=
MAILJET_FROM_EMAIL=cudmore.mb@gmail.com
```

`RECAPTCHA_KEY` is the public score-based reCAPTCHA Enterprise website key, not
a legacy secret or Google API key. Configure its allowed domains (including
`localhost` for development) in the project identified by `GOOGLE_CLOUD_PROJECT`.
Enable the reCAPTCHA Enterprise API and grant the application's service account
the reCAPTCHA Enterprise Agent role (`roles/recaptchaenterprise.agent`).
The server uses Application Default Credentials (ADC). No reCAPTCHA secret is needed.

### Google credentials for local Docker development

Install the Google Cloud CLI if needed, then sign in for ADC:

```bash
brew install --cask gcloud-cli
gcloud auth application-default login
gcloud auth application-default set-quota-project YOUR_GOOGLE_CLOUD_PROJECT
```

Use the same project as `GOOGLE_CLOUD_PROJECT` in `.env`. Your signed-in user must
have permission to create assessments (for example, the reCAPTCHA Enterprise Agent
role) and use the quota project (`serviceusage.services.use`, included in the
Service Usage Consumer role). A project administrator can grant these permissions.
Enable the API if it is not already enabled:

```bash
gcloud services enable recaptchaenterprise.googleapis.com --project YOUR_GOOGLE_CLOUD_PROJECT
```

Docker Compose mounts `~/.config/gcloud/application_default_credentials.json`
read-only into the web container and sets `GOOGLE_APPLICATION_CREDENTIALS` to its
container path. Keep this credential file outside the repository. The file must
exist before starting the web container; Compose will not create it automatically.
After configuring credentials, recreate the web container:

```bash
docker compose up -d --no-deps web
docker compose exec web sh -c 'test -r "$GOOGLE_APPLICATION_CREDENTIALS" && echo "ADC file is readable"'
```

Reload the browser and retry a POST action. If assessment creation still fails,
check `docker compose logs --tail 100 web` for API, project, or permission errors.
These local user credentials are for development only; production should use its
attached service account.

Every frontend POST requests a fresh Enterprise token. All authenticated POST
actions, including logout and admin actions, require a valid token with a matching
route-specific action and a human score of at least `0.5`. GET requests and
anonymous login/signup actions are unchanged on the server. Verification fails
closed in development and production; missing tokens/configuration, API failures,
and rejected assessments display "reCAPTCHA verification failed." to the user.
Failed verification responses also include `score` and `reason` for diagnosis
in the browser Network panel. `score` is `null` when no score is available (for
example, an assessment API or credentials failure); server logs contain the
underlying error. A score below `0.5` returns the reason `low_score`.

Run `go test ./...` to check the backend verification flows. These tests use
mocked Enterprise assessments and do not send tokens or credentials to Google.

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
