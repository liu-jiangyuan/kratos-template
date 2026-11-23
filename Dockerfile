FROM golang:1.25 AS builder

ARG GOPROXY=https://proxy.golang.org,direct
ARG VERSION=dev
ENV GOPROXY=${GOPROXY}

WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build with a stable binary name: `kratos new` renames cmd/server to
# cmd/<project>, but the container entrypoint stays ./server.
RUN CGO_ENABLED=0 make build VERSION=${VERSION}
RUN cp "$$(find ./bin -maxdepth 1 -type f | head -n 1)" /src/server

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates \
		netbase \
		tzdata \
		&& rm -rf /var/lib/apt/lists/ \
		&& apt-get autoremove -y && apt-get autoclean -y \
		&& useradd -r -u 10001 -m appuser

COPY --from=builder --chown=appuser:appuser /src/server /app/server

WORKDIR /app
USER appuser

EXPOSE 8000
VOLUME /data/conf

CMD ["./server", "-conf", "/data/conf"]
