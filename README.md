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

Populate the configured database with demo services, policies, rules, and 6,000
request logs spanning the last 30 days:

```sh
make seed
```

The seed command uses `DATABASE_PATH` from `.env` or the environment, matching
the server; it defaults to `data/openwaf.db`. It replaces the generated demo
logs, rules, and policies for the `*.demo.test` services each time it runs.

Build and run production:

```sh
make build
./bin/openwaf
```

The build generates the static Nuxt frontend and embeds it in the Go executable.
Only `bin/openwaf` is needed to deploy; it serves the UI and API on port 3000.
Build the whole Go package (`go build .`), rather than `go build main.go`, so the
frontend build-tag files are included.

Autonomous investigations API: [docs/investigations-api.md](docs/investigations-api.md).

Country lookup uses the free DB-IP Country Lite database. At startup, OpenWAF
downloads the current monthly release to `data/dbip-country-lite.mmdb` if the
cached database is missing or outdated. It checks hourly for updates and retries
failed downloads. Downloads need no account or API key; request lookups run
entirely locally. The database is neither embedded in the executable nor tracked
in Git. Persist the `data/` directory to reuse it between deployments.

If downloading fails, OpenWAF continues with the previous database, or leaves
country codes unknown until a download succeeds. Private, reserved, and unknown
IPs have no country code. Existing logs are not backfilled.

IP Geolocation by [DB-IP](https://db-ip.com), licensed under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).
