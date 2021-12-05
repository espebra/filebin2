#FROM golang:alpine
#RUN apk add make gcc libc-dev git
FROM golang:buster
RUN export DEBIAN_FRONTEND=noninteractive && apt-get install -y make gcc libc-dev git
RUN go get -u github.com/jstemmer/go-junit-report
RUN go get -u github.com/GeertJohan/go.rice/rice
WORKDIR /app
COPY wait-for-s3.sh .
EXPOSE 8080
CMD make run
