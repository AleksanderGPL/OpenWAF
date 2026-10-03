.PHONY: dev build

dev:
	node scripts/dev.mjs

build:
	cd frontend && npm run generate
	go build -o bin/openwaf .
