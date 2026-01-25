FROM golang:1.24-alpine AS build

ARG VERSION=dev
ARG COMMIT=unknown
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -mod=vendor \
    -trimpath -buildvcs=false \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o filebin2 ./cmd/filebin2

FROM gcr.io/distroless/static:nonroot

COPY --from=build /app/filebin2 /filebin2

EXPOSE 8080

ENTRYPOINT ["/filebin2"]
