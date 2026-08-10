ARG BASE_REGISTRY=docker.io/library

FROM ${BASE_REGISTRY}/golang:1.26-alpine AS backend-build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/registry-proxy ./cmd/registry-proxy

FROM ${BASE_REGISTRY}/node:22-alpine AS frontend-build
ARG APP_VERSION=dev
WORKDIR /app
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
COPY VERSION ./VERSION
RUN VITE_APP_VERSION="$(cat VERSION)" npm run build

FROM ${BASE_REGISTRY}/nginx:1.27-alpine
RUN addgroup -S app && adduser -S -G app app \
    && apk add --no-cache ca-certificates
COPY --from=backend-build /out/api /api
COPY --from=backend-build /out/worker /worker
COPY --from=backend-build /out/registry-proxy /registry-proxy
COPY backend/migrations /migrations
COPY --from=frontend-build /app/dist /usr/share/nginx/html
COPY frontend/nginx.conf /etc/nginx/registrypulse/frontend.conf
COPY deploy/nginx/default.conf /etc/nginx/registrypulse/edge.conf
COPY registrypulse-entrypoint.sh /usr/local/bin/registrypulse-entrypoint
RUN chmod +x /usr/local/bin/registrypulse-entrypoint
ENTRYPOINT ["/usr/local/bin/registrypulse-entrypoint"]
EXPOSE 80
