<div align="center">
  <img src="web/public/logo_text.svg" alt="tcforge" height="60" />
  <p>Containerize your programming contest.</p>

  [![Release](https://img.shields.io/github/v/release/kevincornellius/tcforge?style=flat-square&color=6366f1)](https://github.com/kevincornellius/tcforge/releases)
  [![License](https://img.shields.io/github/license/kevincornellius/tcforge?style=flat-square&color=6366f1)](LICENSE)
  [![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
  [![Docker](https://img.shields.io/badge/Docker-required-2496ED?style=flat-square&logo=docker&logoColor=white)](https://docs.docker.com/get-docker/)
</div>

---

Write test case specs, run one command, get a full contest platform with judge, scoreboard, and web UI — locally or deployed to the cloud.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/kevincornellius/tcforge/main/scripts/install.sh | sh
```

Or download a binary from [Releases](https://github.com/kevincornellius/tcforge/releases).

## Quickstart

```sh
# 1. Set up a new contest
tcforge init

# 2. Edit tcforge.yaml — add problems, accounts, contest settings

# 3. Build test cases (compiles spec.cpp + generates tc/)
tcforge build

# 4. Run locally
tcforge serve
# → http://localhost:6174

# 5. Deploy to the cloud
tcforge deploy --push   # build + push image to GHCR
```

## Requirements

- [Docker](https://docs.docker.com/get-docker/)

## Features

- **One-command setup** — `tcforge init` scaffolds a contest with sample problem
- **Automated test generation** — write `spec.cpp` with tcframe, `tcforge build` does the rest
- **Full judge** — C++17/20 and Python 3, IOI partial scoring, subtasks, live verdict streaming
- **Web UI** — problem statements (HTML/PDF/Markdown), submissions, scoreboard, announcements
- **Cloud deploy** — single `docker buildx` image, runs on Fly.io, Koyeb, Railway, Render, or any VPS

## Deploying

See [docs/deploy.md](docs/deploy.md) for the full deployment guide (Fly.io, Koyeb, Railway, Render, VPS).
