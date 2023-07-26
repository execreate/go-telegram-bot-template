FROM alpine as cacerts

RUN apk update && apk upgrade && apk add --no-cache ca-certificates
RUN update-ca-certificates

FROM golang:1.20 as build

WORKDIR /app
COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download
RUN go mod verify
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/app

FROM scratch
COPY --from=cacerts /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/app /go/bin/app
COPY --from=build /app/locale /app/locale

COPY --from=build /app/static /app/static
ENV MY_BOT_STATIC_CONTENT_PATH=/app/static

CMD ["/go/bin/app", "--locale-path", "/app/locale"]
