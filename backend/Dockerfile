ARG BASE_REGISTRY=docker.io/library
FROM ${BASE_REGISTRY}/golang:1.26-alpine AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/registry-proxy ./cmd/registry-proxy
ARG BASE_REGISTRY=docker.io/library
FROM ${BASE_REGISTRY}/alpine:3.21
RUN addgroup -S app && adduser -S -G app app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /api
COPY --from=build /out/worker /worker
COPY --from=build /out/registry-proxy /registry-proxy
COPY migrations /migrations
USER app:app
ENTRYPOINT ["/api"]
