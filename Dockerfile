FROM golang:1.20 as build

WORKDIR /app
COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download
RUN go mod verify
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/app

FROM alpine as cacerts
RUN apk update && apk upgrade && apk add --no-cache ca-certificates
RUN update-ca-certificates

FROM scratch
COPY --from=cacerts /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/app /go/bin/app
CMD ["/go/bin/app"]
