FROM alpine:3.21 AS cacerts

RUN apk add --no-cache ca-certificates && update-ca-certificates

FROM --platform=$BUILDPLATFORM golang:1.26 AS build

# TARGETOS/TARGETARCH are provided automatically by BuildKit and let us
# cross-compile from the build host's native platform to the target platform
# (e.g. `docker buildx build --platform linux/amd64,linux/arm64`).
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /go/bin/app

FROM scratch
COPY --from=cacerts /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/app /go/bin/app
COPY --from=build /app/locale/*.yaml /app/locale

COPY --from=build /app/static /app/static
ENV MY_BOT_STATIC_CONTENT_PATH=/app/static

CMD ["/go/bin/app", "--locale-path", "/app/locale"]
