FROM golang:1.19.1 as builder
WORKDIR /workspace

COPY . .

# TODO cache doesn't seem to work
RUN \
  --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/mod/pkg \
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o /usr/bin/redis ./app/redis
RUN chmod +x /usr/bin/redis

FROM gcr.io/distroless/cc
WORKDIR /usr/bin

COPY --from=builder /usr/bin/redis .

EXPOSE 6379

ENTRYPOINT [ "/usr/bin/redis", "--port=6379", "--record", "--target=redis:6379" ]
