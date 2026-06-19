FROM golang:1.23-alpine AS judge-builder
WORKDIR /src
COPY go.work go.work.sum* ./
COPY judge/go.mod judge/go.sum* ./judge/
COPY api/go.mod api/go.sum* ./api/
COPY cli/go.mod cli/go.sum* ./cli/
RUN go work sync
COPY judge/ ./judge/
COPY api/ ./api/
COPY cli/ ./cli/
RUN go build -o /bin/judge ./judge

### Final image: isolate sandbox + language runtimes
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    g++ \
    python3 \
    libcap-dev \
    libseccomp-dev \
    git \
    make \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Build isolate from source (IOI sandbox)
RUN git clone https://github.com/ioi/isolate.git /isolate && \
    cd /isolate && \
    make install && \
    rm -rf /isolate

COPY --from=judge-builder /bin/judge /bin/judge

ENTRYPOINT ["/bin/judge"]
