# Slope Stability

Slope Stability is a Go service for modelling a soil slope, its stratigraphic layers and circular slip surfaces. It calculates the factor of safety with Bishop, Fellenius and Janbu methods, records instrument readings, recomputes live pore-pressure effects, and persists events and derived state in SQLite for restart reconciliation.

## Local use

The project uses Go 1.26.3 in module mode.

```bash
GOTOOLCHAIN=local go build ./...
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go vet ./...
GOTOOLCHAIN=local go run . --smoke-test
GOTOOLCHAIN=local go run . -addr :8080
```

The smoke test starts isolated in-process API servers and exits after checking the analysis, monitoring, alert and recovery paths. The normal server stores its SQLite database in `slopestab.db` unless `-db` is supplied.

## Docker

The production `Dockerfile` builds a static binary and runs it directly. It accepts `--smoke-test`:

```bash
docker buildx build --platform linux/amd64 --load -t slope-stability:amd64 .
docker run --rm slope-stability:amd64 --smoke-test
```

`benzhi.Dockerfile` is the Benzhi evaluation image. Build it with the required helper; the first argument is the image name and the optional second argument is the platform:

```bash
./build_benzhi_docker.sh slope-stability-benzhi linux/amd64
docker run -it slope-stability-benzhi:latest
```
