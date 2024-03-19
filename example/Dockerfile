# docker build -t rest-app/example:v0.0.0 .
# docker run --name example -it rest-app/example:v0.0.0 /bin/bash
# docker container stop example
# docker container rm example
# Good reference on building GO images in Docker:
# https://laurent-bel.medium.com/running-go-on-docker-comparing-debian-vs-alpine-vs-distroless-vs-busybox-vs-scratch-18b8c835d9b8
FROM golang:1.21 AS builder
WORKDIR /go/src/github.com/paulfdunn/rest-app/example
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# #RUN go test -v ./... >test.log 2>&1
RUN GOOS=linux GOARCH=arm64 go build

# FROM gcr.io/distroless/static-debian11 AS example
FROM ubuntu:22.04 AS example
# RUN apt-get update
# RUN apt-get install -y sqlite
# RUN apt-get install -y curl
EXPOSE 8000
WORKDIR /app
COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example/example /app/example
COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example/key /app/key
CMD ["./example",  "-https-port=8000", "-log-level=0", "-log-filepath=./example.log", "persistent-directory=/opt/rest-app/data"]
