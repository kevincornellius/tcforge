### Stage 1: build the React frontend
FROM node:20-alpine AS web-builder
WORKDIR /web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

### Stage 2: build the Go API
FROM golang:1.23-alpine AS api-builder
WORKDIR /src
COPY go.work go.work.sum* ./
COPY api/go.mod api/go.sum* ./api/
COPY judge/go.mod judge/go.sum* ./judge/
COPY cli/go.mod cli/go.sum* ./cli/
RUN go work sync
COPY api/ ./api/
COPY judge/ ./judge/
COPY cli/ ./cli/
RUN go build -o /bin/api ./api

### Stage 3: final image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=api-builder /bin/api /bin/api
COPY --from=web-builder /web/dist /app/web/dist
EXPOSE 8080
ENTRYPOINT ["/bin/api"]
