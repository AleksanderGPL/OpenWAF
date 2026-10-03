.PHONY: dev build

dev:
	bun scripts/dev.mjs

build:
	cd frontend && bun run generate
	go build -o bin/openwaf .
