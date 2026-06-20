# Deploying a tcforge Contest

`tcforge deploy` generates all files needed to deploy your contest to any Docker-compatible platform.

## Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_TYPE` | No | `local` | `local` = SQLite on disk. `psql` = Postgres (requires `DATABASE_URL`). |
| `DATABASE_URL` | Only if `DB_TYPE=psql` | — | Postgres connection string (e.g. from Neon). |
| `JWT_SECRET` | No | `set_this_pls` | Secret for signing auth tokens. Users stay logged in across restarts. |

**Recommended for any cloud deploy:**

```sh
DB_TYPE=psql
DATABASE_URL=postgres://user:pass@host/dbname   # from Neon free tier
JWT_SECRET=$(openssl rand -hex 32)              # generate once, save it
```

---

## Deployment options

### Option 1 — Push image to GHCR, deploy anywhere (easiest)

Builds a `linux/amd64` image and pushes it to GitHub Container Registry.
Any platform can then pull and run it — no build step needed on the platform.

**Requirements:** Docker installed, logged into GHCR (`docker login ghcr.io`).

```sh
tcforge deploy --push
# → ghcr.io/<github-user>/<contest>:latest
```

Then on your platform, deploy the image and set the env vars above.

| Platform | How to deploy |
|----------|--------------|
| **Koyeb** | New Service → Docker → paste image URL → Port 8080 |
| **Railway** | New → Deploy Docker Image → paste image URL |
| **Render** | New Web Service → Deploy existing image → Port 8080 |
| **VPS** | `docker run -p 80:8080 -e DATABASE_URL=... -e JWT_SECRET=... <image>` |

---

### Option 2 — Deploy from GitHub repo (no local Docker needed)

Commit the generated files and link your repo. The platform builds the image itself.

```sh
tcforge deploy
git add Dockerfile entrypoint.sh .dockerignore fly.toml
git commit -m "deploy"
git push
```

| Platform | Steps |
|----------|-------|
| **Fly.io** | `tcforge deploy --fly` — fully automatic |
| **Koyeb** | New Service → GitHub → repo → Dockerfile → Port 8080 |
| **Railway** | New → GitHub Repo → repo → Port 8080 |
| **Render** | New Web Service → GitHub → repo → Runtime: Docker → Port 8080 |

Set `DATABASE_URL` and `JWT_SECRET` as environment variables on the platform dashboard.

---

### Option 3 — VPS (run locally on the server)

No containers needed — just install tcforge on the server and run it like local.

```sh
tcforge serve   # starts the contest stack with Docker Compose
```

Data persists on disk automatically. No env vars needed.

---

## Data persistence without Postgres

If you don't set `DATABASE_URL`, tcforge uses SQLite. Mount a disk at `/data` and
tcforge auto-detects it — no config needed.

| Platform | How to add disk at `/data` |
|----------|---------------------------|
| Fly.io | Handled automatically by the generated `fly.toml` |
| Koyeb | Service → Storage → Add volume → mount path `/data` |
| Railway | Service → Volumes → mount path `/data` |
| Render | Service → Disks → mount path `/data` |

Without a disk and without `DATABASE_URL`, data is lost on container restart.
Fine for a quick demo; not suitable for a real contest.
