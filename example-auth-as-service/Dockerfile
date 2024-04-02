# docker build -t rest-app/example-auth-as-service:v0.0.0 .
# docker run --name example-auth-as-service -it rest-app/example-auth-as-service:v0.0.0 /bin/bash
# docker container stop example-auth-as-service
# docker container rm example-auth-as-service
# Good reference on building GO images in Docker:
# https://laurent-bel.medium.com/running-go-on-docker-comparing-debian-vs-alpine-vs-distroless-vs-busybox-vs-scratch-18b8c835d9b8
FROM golang:1.21 AS builder
WORKDIR /go/src/github.com/paulfdunn/rest-app/example-auth-as-service
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# #RUN go test -v ./... >test.log 2>&1
RUN GOOS=linux GOARCH=arm64 go build

# FROM gcr.io/distroless/static-debian11 AS example-auth-as-service
FROM ubuntu:22.04 AS example-auth-as-service
# RUN apt-get update
# RUN apt-get install -y sqlite
# RUN apt-get install -y curl
EXPOSE 8000
WORKDIR /app
COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example-auth-as-service/example-auth-as-service /app/example-auth-as-service
COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example-auth-as-service/key /app/key
CMD ["./example-auth-as-service",  "-https-port=8000", "-log-level=0", "-log-filepath=./example-auth-as-service.log", "persistent-directory=/opt/rest-app/data"]
