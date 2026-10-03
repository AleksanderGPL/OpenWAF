.PHONY: dev build seed

dev:
	bun scripts/dev.mjs

build:
	cd frontend && bun run generate
	go build -o bin/openwaf .

seed:
	go run ./cmd/seed
