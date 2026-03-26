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

**Limits**

- Request body is capped at **2 MiB**; larger bodies receive `413 Request Entity Too Large`.
- XML must be **well-formed**; invalid XML returns `400` with message `invalid XML`.

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

| Variable | Description |
|----------|-------------|
| `CHROME_PATH` | Full path to Chrome/Chromium executable. If unset, common locations and `PATH` are tried (including macOS Google Chrome). |

Example (macOS):

```bash
export CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
```

The Docker image sets `CHROME_PATH=/usr/bin/chromium`.

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

Manifests live under `k8s/local` (namespace `document-service-local`, image `document-service:local`).

**Docker Desktop Kubernetes:** build the image on your machine first, then apply:

```bash
docker build -t document-service:local .
kubectl apply -k k8s/local
kubectl -n document-service-local rollout restart deploy/document-service
kubectl -n document-service-local port-forward svc/document-service 8080:8080
```

## CI/CD

GitHub Actions workflow: [`.github/workflows/ci-cd.yml`](.github/workflows/ci-cd.yml)

- On pull requests to `main`: format check, tests, Go build
- On push to `main` and version tags `v*`: build and push image to `ghcr.io/<owner>/<repo>`

## Project layout

```
.
├── src/cmd/document-service/   # application entrypoint and tests
├── k8s/local/                  # local Kubernetes manifests (Kustomize)
├── Dockerfile
├── docker-compose.yaml
├── go.mod
└── README.md
```

## License

Add a `LICENSE` file if you intend to open-source this repository.
