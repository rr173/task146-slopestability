FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/slopestability .

FROM docker.m.daocloud.io/library/alpine:3.20

WORKDIR /app
COPY --from=build /out/slopestability ./slopestability
ENTRYPOINT ["/app/slopestability"]
