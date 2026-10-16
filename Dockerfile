FROM golang:1.27-alpine AS build

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/wallet-api ./cmd/api

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build /out/wallet-api /usr/local/bin/wallet-api

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/wallet-api"]
