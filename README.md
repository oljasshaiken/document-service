# document-service

Small HTTP service that turns **HTML** or **XML** documents into **PDF** using headless Chromium ([chromedp](https://github.com/chromedp/chromedp)).

## Requirements

- **Go** 1.26+ (see `go.mod`)
- A **Chrome** or **Chromium** binary on the host for local runs (Docker image installs Chromium)

Optional:

- Docker / Docker Compose
- `kubectl` + a local cluster (e.g. Docker Desktop Kubernetes)

## API

Base URL defaults to `http://localhost:8080`.

| Method | Path | Body | Description |
|--------|------|------|-------------|
| `GET` | `/healthz` | — | Liveness/readiness probe; returns `200` and body `ok` |
| `POST` | `/html-to-pdf` | Raw HTML | Renders HTML and returns PDF |
| `POST` | `/xml-to-pdf` | Raw XML | Validates XML, escapes it, wraps in HTML `<pre>`, returns PDF |
| `GET` | `/metrics` | — | [Prometheus](https://prometheus.io/) metrics for scraping |

**Limits**

- Request body size defaults to **2 MiB** (configurable); larger bodies receive `413 Request Entity Too Large`.
- XML must be **well-formed** (strict decoder); invalid XML returns `400` with message `invalid XML`.
- Concurrent PDF renders are **bounded**; extra requests **wait** for a slot. If the client disconnects while waiting, the server responds with **`408 Request Timeout`**. Other failures while acquiring a slot may yield **`503 Service Unavailable`**.

**Response (success)**

- `200 OK`
- `Content-Type: application/pdf`
- `Content-Disposition: attachment; filename="document.pdf"`

### Examples

**Health**

```bash
curl -sS http://localhost:8080/healthz
```

**HTML → PDF**

```bash
curl -sS -o out.pdf \
  -X POST "http://localhost:8080/html-to-pdf" \
  -H "Content-Type: text/html; charset=utf-8" \
  --data '<!doctype html><html><body><h1>Hello</h1></body></html>'
```

**XML → PDF**

```bash
curl -sS -o out-xml.pdf \
  -X POST "http://localhost:8080/xml-to-pdf" \
  -H "Content-Type: application/xml; charset=utf-8" \
  --data '<note><to>Tove</to><body>Hello</body></note>'
```

## Configuration

All service tuning variables use the `DOCUMENT_SERVICE_*` prefix (except `CHROME_PATH`, which Chromium expects).

| Variable | Default | Description |
|----------|---------|-------------|
| `CHROME_PATH` | *(auto)* | Path to Chrome/Chromium. The Docker image sets `/usr/bin/chromium`. |
| `DOCUMENT_SERVICE_ADDR` | `:8080` | Listen address. |
| `DOCUMENT_SERVICE_READ_HEADER_TIMEOUT` | `5s` | `ReadHeaderTimeout`. |
| `DOCUMENT_SERVICE_READ_TIMEOUT` | `10s` | `ReadTimeout`. |
| `DOCUMENT_SERVICE_WRITE_TIMEOUT` | `60s` | `WriteTimeout`. |
| `DOCUMENT_SERVICE_IDLE_TIMEOUT` | `120s` | `IdleTimeout`. |
| `DOCUMENT_SERVICE_SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown deadline on SIGINT/SIGTERM. |
| `DOCUMENT_SERVICE_MAX_BODY_BYTES` | `2097152` | Max request body size (bytes). |
| `DOCUMENT_SERVICE_PDF_TIMEOUT` | `30s` | Per-render Chromium timeout. |
| `DOCUMENT_SERVICE_MAX_CONCURRENT_RENDERS` | `2` | Maximum concurrent PDF jobs (protects memory/CPU). |
| `DOCUMENT_SERVICE_API_KEYS` | *(empty)* | Comma-separated API keys. If set, `POST /html-to-pdf` and `POST /xml-to-pdf` require `X-API-Key` or `Authorization: Bearer <key>`. `/healthz` and `/metrics` stay open. |
| `DOCUMENT_SERVICE_RATE_LIMIT_RPS` | `0` (off) | Per–client-IP rate limit (requests/sec). `X-Forwarded-For` first hop is trusted when present. |
| `DOCUMENT_SERVICE_RATE_LIMIT_BURST` | `20` | Token bucket burst for rate limiting. |
| `DOCUMENT_SERVICE_LOG_JSON` | `false` | If `true`, logs are JSON (`slog`). |

Example (macOS):

```bash
export CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
export DOCUMENT_SERVICE_API_KEYS="dev-key-1,dev-key-2"
```

The process shuts down gracefully: it stops accepting new connections and drains in-flight work up to `DOCUMENT_SERVICE_SHUTDOWN_TIMEOUT`.

## Run locally

```bash
go run ./src/cmd/document-service
```

Or build a binary:

```bash
go build -o document-service ./src/cmd/document-service
./document-service
```

## Tests

```bash
go test ./...
```

## Docker

Build and run:

```bash
docker build -t document-service:local .
docker run --rm -p 8080:8080 document-service:local
```

Or use Compose:

```bash
docker compose up --build
```

## Kubernetes (local)

Manifests live under [`k8s/local`](k8s/local) (namespace `document-service-local`, image `document-service:local`).

**Docker Desktop Kubernetes:** build the image on your machine first, then apply:

```bash
docker build -t document-service:local .
kubectl apply -k k8s/local
kubectl -n document-service-local rollout restart deploy/document-service
kubectl -n document-service-local port-forward svc/document-service 8080:8080
```

## Kubernetes (production-oriented)

[`k8s/production`](k8s/production) provides a starting point: namespace `document-service`, **2** replicas, **GHCR** image `ghcr.io/oljasshaiken/document-service:latest`, non-root `securityContext`, PodDisruptionBudget, and higher resource requests/limits.

```bash
kubectl apply -k k8s/production
```

Set `DOCUMENT_SERVICE_API_KEYS` (and other env) via `Secret`/`ConfigMap` patches before relying on this in a real cluster. Use an **Ingress** with TLS termination in front of the Service.

## CI/CD

GitHub Actions workflow: [`.github/workflows/ci-cd.yml`](.github/workflows/ci-cd.yml)

- On pull requests to `main`: format check, tests, Go build, **Trivy filesystem scan** (report-only)
- On push to `main` and version tags `v*`: build and push image to `ghcr.io/<owner>/<repo>`

## Project layout

```
.
├── src/cmd/document-service/   # application entrypoint and tests
├── k8s/local/                  # local Kubernetes manifests (Kustomize)
├── k8s/production/             # example production-oriented manifests
├── Dockerfile
├── docker-compose.yaml
├── go.mod
└── README.md
```

## License

Add a `LICENSE` file if you intend to open-source this repository.
