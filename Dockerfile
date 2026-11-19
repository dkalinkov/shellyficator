# Base images are pinned by digest; Dependabot proposes updates.
# Build stages run on the build machine and cross-compile for the target
# platform, so only the small runtime stage needs emulation.
FROM --platform=$BUILDPLATFORM node:26-alpine@sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5 AS styles
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
COPY ui ./ui
COPY scripts ./scripts
COPY internal/web ./internal/web
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
ARG TARGETOS
ARG TARGETARCH
ARG BUILD_TYPE=dev
ARG BUILD_COMMIT=unknown
ARG BUILD_DATE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=styles /src/internal/web/static ./internal/web/static
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH sh scripts/build.sh /shellyficator

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
ARG BUILD_COMMIT=unknown
ARG BUILD_DATE
LABEL org.opencontainers.image.title="Shellyficator" \
      org.opencontainers.image.description="Self-hosted dashboard for Shelly Gen2+ device inventory and configuration drift" \
      org.opencontainers.image.source="https://github.com/dkalinkov/shellyficator" \
      org.opencontainers.image.licenses="MIT" \
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
