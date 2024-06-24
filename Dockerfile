FROM golang:bullseye
RUN export DEBIAN_FRONTEND=noninteractive && apt-get install -y make gcc libc-dev git
RUN go install github.com/jstemmer/go-junit-report/v2@latest
WORKDIR /app
COPY wait-for-s3.sh .
EXPOSE 8080
CMD make run
