FROM golang:1.19.1 as builder
WORKDIR /workspace

COPY . .

RUN \
  --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/mod/pkg \
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o /usr/bin/postgres ./app/postgres
RUN chmod +x /usr/bin/postgres

FROM gcr.io/distroless/cc
WORKDIR /usr/bin

COPY --from=builder /usr/bin/postgres .

EXPOSE 5432

ENTRYPOINT [ "/usr/bin/postgres", "--port=5432", "--target=postgres:5432" ]
