# tcforge

Containerize your programming contest. Write test case specs, run one command, get a full contest platform with judge, scoreboard, and web UI — locally or deployed to the cloud.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/kevincornellius/tcforge/main/scripts/install.sh | sh
```

Or download a binary from [Releases](https://github.com/kevincornellius/tcforge/releases).

## Usage

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
tcforge deploy          # generates deploy files + instructions
tcforge deploy --push   # build + push image to GHCR
```

## Requirements

- [Docker](https://docs.docker.com/get-docker/)

## Deploying

See [docs/deploy.md](docs/deploy.md) for full deployment guide (Fly.io, Koyeb, Railway, Render, VPS).
