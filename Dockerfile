FROM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS build

WORKDIR /app

COPY . .

RUN go build -o clear-renovate-notifications .

FROM alpine:latest@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

LABEL org.opencontainers.image.source=https://github.com/wajeht/clear-renovate-notifications

RUN apk --no-cache add ca-certificates curl

RUN addgroup -g 1000 -S app && adduser -S app -u 1000 -G app

WORKDIR /app

COPY --from=build /app/clear-renovate-notifications ./clear-renovate-notifications

USER app

EXPOSE 80

HEALTHCHECK CMD curl -f http://localhost/healthz || exit 1

CMD ["./clear-renovate-notifications"]
