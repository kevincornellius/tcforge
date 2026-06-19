# tcforge

Local contest hosting tool powered by tcframe. One command to compile test cases, boot a full judge + web stack, and share via tunnel.

## Structure

```
cli/      # Go CLI binary (tcforge init/build/serve/stop/push)
api/      # Go REST + WebSocket backend
judge/    # Go judge worker + isolate sandbox
web/      # React + Vite frontend
docker/
  builder/  # g++ + tcframe image (no app code)
scripts/
  install.sh
```

## Dev

```bash
go build -o tcforge ./cli   # build CLI binary
go work sync                # sync workspace dependencies
```

## Images (ghcr.io/kevincornellius/)

| Image            | Built from      |
|------------------|-----------------|
| tcforge-builder  | docker/builder/ |
| tcforge-api      | api/Dockerfile  |
| tcforge-judge    | judge/Dockerfile|

Build all from repo root:
```bash
docker build -t ghcr.io/kevincornellius/tcforge-builder:latest ./docker/builder
docker build -t ghcr.io/kevincornellius/tcforge-api:latest    -f api/Dockerfile .
docker build -t ghcr.io/kevincornellius/tcforge-judge:latest  -f judge/Dockerfile .
```

## Key decisions

- `tcforge build` runs tcframe inside Docker — user needs no local g++ or tcframe
- `tcforge serve` generates `.tcforge/docker-compose.yml` and runs it
- `tcforge.yaml` is source of truth for contest config and accounts
- SQLite at `.tcforge/db.sqlite` for runtime data (submissions, scores)
- Judgels migration via SSH (`tcforge push --judgels`)
- Windows supported via WSL2 only
