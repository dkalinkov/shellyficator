FROM node:24-alpine AS styles
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
COPY ui ./ui
COPY scripts ./scripts
COPY internal/web ./internal/web
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=styles /src/internal/web/static ./internal/web/static
RUN CGO_ENABLED=0 go build -trimpath -o /shelly-manager .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 shelly && adduser -D -u 10001 -G shelly shelly && mkdir /data && chown shelly:shelly /data
COPY --from=build /shelly-manager /usr/local/bin/shelly-manager
USER shelly
WORKDIR /data
VOLUME /data
EXPOSE 8090
ENTRYPOINT ["shelly-manager"]
CMD ["serve", "--http=0.0.0.0:8090", "--dir=/data"]
