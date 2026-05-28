FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/nikki ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends postgresql-client wget \
  && rm -rf /var/lib/apt/lists/* \
  && useradd -r -u 10001 nikki \
  && mkdir -p /data /uploads \
  && chown -R nikki:nikki /data /uploads

USER nikki
WORKDIR /app

COPY --from=build /out/nikki /app/nikki

ENV NIKKI_ADDR=:8080
ENV NIKKI_UPLOAD_DIR=/uploads
ENV NIKKI_PUBLIC_UPLOAD_BASE=/uploads

EXPOSE 8080

CMD ["/app/nikki"]
