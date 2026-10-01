.PHONY: build run test css dev dev-build docker

css:
	npm ci --ignore-scripts
	npm run build

build: css
	sh scripts/build.sh bin/shellyficator

docker:
	sh scripts/build.sh docker

run:
	go run . serve --http=127.0.0.1:8090 --dir=./pb_data

node_modules/.package-lock.json: package.json package-lock.json
	npm ci --ignore-scripts

dev: node_modules/.package-lock.json
	go tool air -c .air.toml

dev-build: node_modules/.package-lock.json
	npm run build
	sh scripts/build.sh .air/shellyficator

test:
	go test ./...
	go vet ./...
