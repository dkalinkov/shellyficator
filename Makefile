.PHONY: build run test fuzz css dev dev-build docker format format-check

CSS_INPUTS := ui/styles.css internal/web/templates internal/web/templates.go \
	$(wildcard internal/web/templates/*.html) \
	$(filter-out internal/web/static/htmx.min.js,$(wildcard internal/web/static/*.js))
VENDOR_ASSETS := internal/web/static/htmx.min.js \
	internal/web/static/htmx.LICENSE \
	internal/web/static/fonts.LICENSE \
	internal/web/static/plex-400.woff2 \
	internal/web/static/plex-500.woff2 \
	internal/web/static/plex-600.woff2 \
	internal/web/static/archivo-900.woff2 \
	internal/web/static/archivo.LICENSE

css: internal/web/static/app.css $(VENDOR_ASSETS)

# Tailwind skips writing identical CSS; touch marks a successful build current.
internal/web/static/app.css: $(CSS_INPUTS) node_modules/.package-lock.json
	npm run build:css
	@touch $@

# One vendor run produces every file, including when an output is missing.
$(VENDOR_ASSETS) &: scripts/vendor.mjs node_modules/.package-lock.json
	npm run vendor

build: css
	sh scripts/build.sh bin/shellyficator

docker:
	sh scripts/build.sh docker

run: css
	go run . serve --http=127.0.0.1:8090 --dir=./pb_data

node_modules/.package-lock.json: package.json package-lock.json
	npm ci --ignore-scripts

dev: node_modules/.package-lock.json
	go tool air -c .air.toml

dev-build: css
	BUILD_TYPE=dev BUILD_DATE="$${BUILD_DATE:-unknown}" sh scripts/build.sh .air/shellyficator

test:
	go test ./...
	go vet ./...

FUZZTIME ?= 30s
fuzz:
	sh scripts/fuzz.sh $(FUZZTIME)

format: node_modules/.package-lock.json
	npm run format:templates

format-check: node_modules/.package-lock.json
	npm run check:templates
