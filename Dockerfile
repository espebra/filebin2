FROM golang:alpine
RUN apk add make gcc libc-dev
WORKDIR /app
EXPOSE 8888
#CMD make check
CMD sleep 100000000