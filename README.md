# Shellyficator

Requires Go 1.27. Development and CSS builds also need Node.js 24.

## Run

```sh
cp .env.example .env
make run
```

Edit `.env` as needed, then open [localhost:8090](http://localhost:8090) and create an account. Data admin: `/_/`.

Use `make dev` for automatic restarts. Refresh the browser after CSS or JavaScript changes.

## Build and test

```sh
make build
make test
./bin/shellyficator --version
./bin/shellyficator serve --http=127.0.0.1:8090 --dir=./pb_data
```

[VERSION](VERSION) holds the app version. See [configuration and releases](docs/operations.md) for dotenv precedence and the release workflow.

## Docker on Linux

```sh
make docker
docker run --rm --name shellyficator \
  --env-file .env \
  --network=host \
  -v shellyficator-data:/data \
  shellyficator:local serve --http=127.0.0.1:8090 --dir=/data
```

Preserve `pb_data/` or the Docker data volume across upgrades; reuse the existing volume when upgrading an older install. Use `make run` for Docker Desktop development.

[Scope](PRODUCT_BRIEF.md) · [Status and next steps](IMPLEMENTATION.md) · [Network research](research/network-discovery-2026-09-30.md)
