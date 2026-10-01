.PHONY: build run test css dev dev-build

css:
	npm ci --ignore-scripts
	npm run build

build: css
	CGO_ENABLED=0 go build -trimpath -o bin/shelly-manager .

run:
	go run . serve --http=127.0.0.1:8090 --dir=./pb_data

node_modules/.package-lock.json: package.json package-lock.json
	npm ci --ignore-scripts

dev: node_modules/.package-lock.json
	go tool air -c .air.toml

dev-build: node_modules/.package-lock.json
	npm run build
	CGO_ENABLED=0 go build -trimpath -o .air/shelly-manager .

test:
	go test ./...
	go vet ./...
