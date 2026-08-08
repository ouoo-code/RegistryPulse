ARG BASE_REGISTRY=docker.io/library
FROM ${BASE_REGISTRY}/node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build
ARG BASE_REGISTRY=docker.io/library
FROM ${BASE_REGISTRY}/nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
