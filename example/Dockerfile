# docker build -t rest-app/example:v0.0.0 .
# docker login --username paulfdunn
# docker push paulfdunn/rest-app/example:v0.0.0
# docker run --name example --network host -it rest-app/example:v0.0.0 /bin/ash
# docker container stop example
# docker container rm example

###########################################################################
# 2 stage with Alpine - image size: 56MB
###########################################################################
# FROM golang:alpine as builder
# COPY ./ /go/src/github.com/paulfdunn/rest-app/example
# WORKDIR /go/src/github.com/paulfdunn/rest-app/example
# RUN go mod download
# #RUN go test -v ./... >test.log 2>&1
# # https://github.com/mattn/go-sqlite3#alpine
# RUN apk add --update gcc musl-dev
# RUN GOOS=linux GOARCH=arm64 go build

# FROM alpine:latest AS example
# RUN apk --no-cache add sqlite
# RUN apk --no-cache add curl
# # add perl-utils for json_pp
# RUN apk --no-cache add perl-utils
# EXPOSE 8000
# WORKDIR /app
# COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example/example /app/example
# COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example/key /app/key
# # DO NOT use "-log-filepath=" with Docker, just let it go to STDOUT.
# CMD ["./example",  "-https-port=8000", "-log-level=0", "persistent-directory=/opt/rest-app/data"]

###########################################################################
# 2 stage with Ubuntu - image size: 178 MB
# First stage only results in 609MB container.
###########################################################################
FROM bitnami/golang:1.21.8-debian-12-r0 AS builder
RUN apt-get update
COPY ./ /go/src/github.com/paulfdunn/rest-app/example
WORKDIR /go/src/github.com/paulfdunn/rest-app/example
# GOPROXY is not set correctly with this image; fix it.
RUN go env -w GOPROXY='https://proxy.golang.org,direct'
RUN go mod download
#RUN go test -v ./... >test.log 2>&1
RUN GOOS=linux GOARCH=arm64 go build

# New stage to create application container
FROM ubuntu:latest AS example
RUN apt-get update
RUN apt-get install -y sqlite
RUN apt-get install -y curl
# add perl for json_pp
RUN apt-get install -y perl
EXPOSE 8000
WORKDIR /app
COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example/example /app/example
COPY --from=builder /go/src/github.com/paulfdunn/rest-app/example/key /app/key
# DO NOT use "-log-filepath=" with Docker, just let it go to STDOUT.
CMD ["./example",  "-https-port=8000", "-log-level=0", "-log-filepath=./example.log"]
