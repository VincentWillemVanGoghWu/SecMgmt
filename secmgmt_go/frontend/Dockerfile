ARG DOCKER_LIBRARY_MIRROR=docker.m.daocloud.io/library
FROM ${DOCKER_LIBRARY_MIRROR}/node:22-bookworm AS builder

WORKDIR /src/frontend

ARG NPM_REGISTRY=https://registry.npmmirror.com
RUN npm config set registry "${NPM_REGISTRY}"

COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY . ./

ARG VITE_API_BASE_URL=/api
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

RUN rm -rf dist \
    && npm run build \
    && test -f dist/index.html

FROM ${DOCKER_LIBRARY_MIRROR}/nginx:1.27-alpine

COPY --from=builder /src/frontend/dist/ /usr/share/nginx/html/

EXPOSE 80
