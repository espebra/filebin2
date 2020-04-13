#FROM golang:alpine
#RUN apk add make gcc libc-dev git
FROM golang:buster
RUN apt install make gcc libc-dev git
RUN go get -u github.com/jstemmer/go-junit-report
RUN go get -u github.com/GeertJohan/go.rice/rice
WORKDIR /app
EXPOSE 8888
CMD make
