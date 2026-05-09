# FROM centos:8
FROM golang:latest
MAINTAINER SmartBrave <SmartBraveCoder@gmail.com>

#build command: docker build -t sbraveyoung/rtmp_server:latest .
#run command: docker run --rm --name rtmp_server -p 1935:1935 -p 8080:8080 sbraveyoung/rtmp_server:latest

COPY . $GOPATH/src/github.com/sbraveyoung/GGmpeg
WORKDIR $GOPATH/src/github.com/sbraveyoung/GGmpeg

# RUN /usr/local/go/bin/go build src/main.go
RUN go build demo/rtmp_server.go

CMD ./rtmp_server
