# docker build -t rest-app/example-standalone:v0.0.0 .
# docker run --name example-standalone -it rest-app/example-standalone:v0.0.0 /bin/bash
# docker container stop example-standalone
# docker container rm example-standalone
# Good reference on building GO images in Docker:
# https://laurent-bel.medium.com/running-go-on-docker-comparing-debian-vs-alpine-vs-distroless-vs-busybox-vs-scratch-18b8c835d9b8
FROM golang:1.21 AS builder
WORKDIR /go/src/github.com/paulfdunn/rest-app/example-standalone
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# #RUN go test -v ./... >test.log 2>&1
RUN GOOS=linux GOARCH=arm64 go build

# FROM gcr.io/distroless/static-debian11 AS example-standalone
FROM ubuntu:22.04 AS example-standalone
# RUN apt-get update
# RUN apt-get install -y sqlite
# RUN apt-get install -y curl
EXPOSE 8000
WORKDIR /app
COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example-standalone/example-standalone /app/example-standalone
COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example-standalone/key /app/key
CMD ["./example-standalone",  "-https-port=8000", "-log-level=0", "-log-filepath=./example-standalone.log", "persistent-directory=/opt/rest-app/data"]
