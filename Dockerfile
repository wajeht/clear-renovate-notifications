FROM golang:1.26-alpine@sha256:f1ddd9fe14fffc091dd98cb4bfa999f32c5fc77d2f2305ea9f0e2595c5437c14 AS build

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
