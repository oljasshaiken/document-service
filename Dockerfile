FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/document-service ./src/cmd/document-service

FROM alpine:3.21

RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    ttf-freefont

ENV CHROME_PATH=/usr/bin/chromium

RUN adduser -D -u 65532 appuser

WORKDIR /tmp
COPY --from=builder --chown=65532:65532 /bin/document-service /usr/local/bin/document-service

USER appuser

EXPOSE 8080

CMD ["document-service"]
