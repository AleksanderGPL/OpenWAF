# OpenWAF

Install Go (the version in `go.mod`), Node.js, and Make, then install frontend
dependencies with `cd frontend && npm install` (or `bun install`).

Start development from the repository root:

```sh
make dev
```

Open http://localhost:3000. Nuxt provides hot reload and proxies `/api` requests
to the Go backend on 127.0.0.1:3001. The development Go build does not embed
frontend assets, so no Nuxt build is needed. Ctrl-C stops both servers; if either
server exits, the other is stopped too. Restart `make dev` after Go changes.
You can also run `node scripts/dev.mjs` directly.

Build and run production:

```sh
make build
./bin/openwaf
```

The build generates the static Nuxt frontend and embeds it in the Go executable.
Only `bin/openwaf` is needed to deploy; it serves the UI and API on port 3000.
Build the whole Go package (`go build .`), rather than `go build main.go`, so the
frontend build-tag files are included.
