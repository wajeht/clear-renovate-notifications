FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /app

COPY . .

RUN go build -o clear-renovate-notifications .

FROM alpine:latest@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

LABEL org.opencontainers.image.source=https://github.com/wajeht/clear-renovate-notifications

RUN apk --no-cache add ca-certificates curl

RUN addgroup -g 1000 -S app && adduser -S app -u 1000 -G app

WORKDIR /app

COPY --from=build /app/clear-renovate-notifications ./clear-renovate-notifications

USER app

EXPOSE 80

HEALTHCHECK CMD curl -f http://localhost/healthz || exit 1

CMD ["./clear-renovate-notifications"]
