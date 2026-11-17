FROM node:24-alpine AS styles
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
COPY ui ./ui
COPY scripts ./scripts
COPY internal/web ./internal/web
RUN npm run build

FROM golang:1.27-alpine AS build
ARG BUILD_TYPE=dev
ARG BUILD_COMMIT=unknown
ARG BUILD_DATE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=styles /src/internal/web/static ./internal/web/static
RUN sh scripts/build.sh /shellyficator

FROM alpine:3.24
ARG BUILD_COMMIT=unknown
ARG BUILD_DATE
LABEL org.opencontainers.image.title="Shellyficator" \
      org.opencontainers.image.revision="$BUILD_COMMIT" \
      org.opencontainers.image.created="$BUILD_DATE"
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 shellyficator \
    && adduser -D -u 10001 -G shellyficator shellyficator \
    && mkdir /data \
    && chown shellyficator:shellyficator /data
COPY --from=build /shellyficator /usr/local/bin/shellyficator
USER shellyficator
WORKDIR /data
VOLUME /data
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8090/healthz || exit 1
ENTRYPOINT ["shellyficator"]
CMD ["serve", "--http=0.0.0.0:8090", "--dir=/data"]
