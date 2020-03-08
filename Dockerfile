FROM golang:alpine
RUN apk add make gcc libc-dev git
RUN go get -u github.com/jstemmer/go-junit-report
WORKDIR /app
EXPOSE 8888
CMD make
