# FROM centos:8
FROM golang:latest
MAINTAINER SmartBrave <SmartBraveCoder@gmail.com>

#build command: docker build -t smartbrave/rtmp_server:latest .
#run command: docker run --rm --name rtmp_server -p 1935:1935 -p 8080:8080 smartbrave/rtmp_server:latest

COPY . $GOPATH/src/github.com/SmartBrave/GGmpeg
WORKDIR $GOPATH/src/github.com/SmartBrave/GGmpeg

# RUN /usr/local/go/bin/go build src/main.go
RUN go build demo/rtmp_server.go

CMD ./rtmp_server
