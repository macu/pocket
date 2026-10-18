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
POST_IMAGE_BUCKET=
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

New topics, posts, post edits, and user handles and display names (on signup and
account updates) are checked with the OpenAI moderation API
(`omni-moderation-latest`). Profile display names are trimmed, repeated spaces
are collapsed, and non-printable characters are rejected. Passwords are not trimmed.
Set `OPENAI_SECRET_KEY` to enable moderation; when unset,
text moderation is skipped locally and requests fail in App Engine.
Image moderation is always required, including locally, before any image is uploaded.

## Post images

Posts support one public image, with or without text; completely empty posts are
rejected. Upload JPEG, PNG, or static WebP files up to 10 MiB and 20 megapixels,
at least 100 x 100 pixels. Animated PNG/WebP and other formats are rejected.
The server applies JPEG orientation, fits images within 1200 x 1200 without upscaling, strips metadata,
and preserves the format (PNG transparency is retained). Every upload gets a
new UUID filename. The processed pixels are checked by OpenAI moderation before
uploading to Google Cloud Storage.

Set `POST_IMAGE_BUCKET` in `.env` and `app.yaml` to a dedicated image bucket. Enable
`storage.googleapis.com` and give the app's service account
`roles/storage.objectAdmin` on this dedicated bucket. Images are public through
an `allUsers` binding for `roles/storage.objectViewer`; do not store private
files in this bucket. Local uploads use Application Default Credentials with
the same bucket permission and require `OPENAI_SECRET_KEY`.

Apply `sql/migrations/20261018_post_images.pgsql` before deploying. Saving an
edit can remove or replace the current image; "Restore current image" discards
that pending change. Revisions store only text and an "Image added", "Image
updated", or "Image deleted" flag, never previous images or image URLs.

Deploy the cleanup schedule alongside the app:

```bash
gcloud app deploy app.yaml cron.yaml
```

Database triggers queue replaced/deleted images, including images in deleted
sub-posts. The cron job removes up to 20 queued objects every 15 minutes and
retries failures. Abandoned uploads are eligible after one hour. Cloud Storage
and browser caches can retain deleted images temporarily (cache lifetime is one
hour); bucket soft-delete retention may also apply. Monitor cleanup failures
in the application logs.

## reCAPTCHA and local testing

Leave `RECAPTCHA_KEY` empty in your local `.env` to bypass reCAPTCHA in both the
browser and server. This bypass is only available in local mode (`APP_ENGINE`
unset or not `1`); App Engine still requires a configured key and verification.

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

When a key is configured, POSTs protected by reCAPTCHA require a matching token
action and score of at least `0.5`, including locally. Failed responses include
`score` and `reason`; check the browser
Network panel and `docker compose logs --tail 100 web` for diagnostics.
Production uses its attached service account, not local ADC credentials.

## Start up containers

```bash
cd pocket
sh ./bin/restart-all.sh
```

## Init DB and access postgres interactive shell

Run `sql/init.pgsql` by passing contents into psql:

```bash
sh ./bin/psql-execute-script.sh < sql/init.pgsql
sh ./bin/psql-shell.sh
```

For an existing database, apply any SQL scripts in `sql/migrations/` before
deploying the updated application.

The `20261017_case_insensitive_email.pgsql` migration enforces case-insensitive
email uniqueness, including the domain. Resolve any existing accounts whose
emails differ only by case before applying it; the migration fails without
changing the index if such duplicates exist.

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

## Deleting posts

Authors can delete their own posts from the edit page at any time. Deletion also
removes sub-posts, votes, and revisions and cannot be undone. Administrators can
delete posts from the post page using the admin-marked Delete button.

## Test in browser

http://localhost:2026/

## Deploy to Google App Engine

`app.yaml` is a local deployment configuration excluded from Git. Keep it in
the repository root for `gcloud app deploy`; it is not included in a fresh clone.
Do not commit API credentials or other secrets.

Create `app.yaml` using this skeleton. Fill in the empty strings with your
deployment-specific values; keep the runtime, build target, and handlers as shown.

```yaml
runtime: go127

build_env_variables:
  GOOGLE_BUILDABLE: ./cmd

beta_settings:
  cloud_sql_instances: ""

handlers:
- url: /css
  static_dir: css
  secure: always
- url: /js
  static_dir: js
  secure: always
- url: /.*
  script: auto
  secure: always

env_variables:
  APP_ENGINE: "1"
  DB_HOST: ""
  DB_USER: ""
  DB_NAME: ""
  DB_PASS_SECRET: ""
  DOMAIN: ""
  POST_IMAGE_BUCKET: ""
  VERSION_STAMP: 20261016204052
  RECAPTCHA_KEY: ""
  OPENAI_SECRET_KEY_SECRET: ""
  MAILJET_API_KEY_SECRET: ""
  MAILJET_SECRET_SECRET: ""
  MAILJET_FROM_EMAIL: ""
```

Run `sh rebuild.sh` before deploying to rebuild the frontend and refresh
`VERSION_STAMP` in both `.env` and `app.yaml`. The stamp is appended as `?v=`
to the compiled CSS and JavaScript URLs for cache busting.

The app uses the App Engine standard environment with Go 1.27 and PostgreSQL on
Cloud SQL. Set `cloud_sql_instances` to `PROJECT_ID:REGION:INSTANCE_ID` and
`DB_HOST` to `/cloudsql/PROJECT_ID:REGION:INSTANCE_ID`. Set `DOMAIN` to the public
hostname and `RECAPTCHA_KEY` to the production reCAPTCHA site key. Set `DB_USER`
and `DB_NAME` to your database user and database name. Initialize the database
with `sql/init.pgsql`, and store the
database user's password in Secret Manager at
`projects/PROJECT_ID/secrets/SECRET_NAME`; set `DB_PASS_SECRET` to that resource
name.
Create Secret Manager secrets with the resource names configured for
`OPENAI_SECRET_KEY_SECRET`, `MAILJET_API_KEY_SECRET`, and
`MAILJET_SECRET_SECRET` in `app.yaml`, and add their corresponding API
credentials as secret versions. The App Engine service account needs Secret
Manager Secret Accessor on each secret. Set `MAILJET_FROM_EMAIL` to an address
verified with Mailjet.

Enable the App Engine, Cloud SQL Admin, and Secret Manager APIs. Grant the
App Engine service account the Cloud SQL Client and Secret Manager Secret
Accessor roles for the configured secrets, then deploy from the repository
root:

```bash
gcloud app deploy
```

`app.yaml` configures Cloud SQL's Unix socket, HTTPS, and static CSS/JS
handlers. `.gcloudignore` excludes local environment files and development
artifacts from uploads. The App Engine service account also needs access to any
other Google Cloud APIs used by the app. Set the production reCAPTCHA,
Mailjet, and OpenAI configuration before relying on features that use them;
the secret resource names and non-secret sender address are configured in
`app.yaml`, while API credentials remain in Secret Manager.
