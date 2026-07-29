# update-ip contributor guidance

## Scope

These instructions apply to this Go module and every file below this directory.

## Current project context

- This Go module replaces the legacy implementation and targets Go 1.22. Its direct dependencies are Zap and `yaml.v3`; the legacy `../update-ip.py` entry point now forwards its arguments to the built `update-ip/update-ip` binary.
- `cmd/update-ip/main.go` is the small executable entry point. `internal/updateip/app.go` loads configuration, creates separate IPv6 and IPv4 detectors and the Yandex client, applies a 70-second run timeout, and invokes `Updater.Run`.
- `internal/updateip/config.go` owns the five-field YAML model and validation. `configs/update_ip.yaml` is the compatibility fixture and example.
- `internal/updateip/detector.go` owns address-family binding and HTTPS discovery. Discovery bypasses environment proxies, rejects redirects, filters private and special-purpose ranges, and tries configured services in order when a service fails or returns an unusable address.
- `internal/updateip/updater.go` owns provider-independent orchestration. It retains at most one valid public address per family and independently synchronizes every available IPv6 and IPv4 record during the same run; per-family synchronization errors are joined after both families are attempted.
- `internal/updateip/yandex.go` implements `DNSProvider` through the Yandex 360 DNS endpoint on `api360.yandex.net`. HTTP execution stays behind `HTTPDoer`; request construction and response validation remain separate.
- Tests use in-package stubs and `httptest.Server`; they must never call real address or DNS services.
- Runtime flow is: load and validate config; detect up to one public IPv6 and IPv4 address; list records once; independently skip an equal canonical address or add/update each available family record; emit structured Zap fields; join any per-family update errors.

## Design priorities

- Prefer simple, readable Go over speculative abstractions. Apply DRY only when duplication represents the same domain rule.
- Use descriptive, searchable names. Replace unexplained limits, timeouts, record types, and service URLs with named constants.
- Keep functions small and focused on one level of abstraction. Extract a helper when it gives a behavior or boundary condition a clear name.
- Keep interfaces at integration boundaries. Inject address detectors, DNS providers, HTTP clients, and Zap loggers so behavior remains testable.
- Prefer standard-library value types such as `net/netip.Addr` over raw strings for IP-address logic.
- Return contextual, lower-case errors and wrap causes with `%w`. Never log and return the same error.
- Use structured Zap fields. Do not add `printf`-style logging wrappers.
- Add idiomatic Go doc comments for exported identifiers and the package. Document unexported functions only when they encode non-obvious policy or boundary behavior; avoid narrating obvious code.

## Behavioral invariants

- Preserve the YAML keys `token`, `organization`, `domain`, `subdomain`, and `ttl`, and keep `/etc/update-ip.yaml` as the default path.
- Parse configuration with the direct `yaml.Unmarshal` API. Do not introduce a generalized configuration layer for this five-field format.
- External address detection must use HTTPS, bound to the requested address family: `tcp6` with `::` for IPv6 and `tcp4` with `0.0.0.0` for IPv4. Discovery connections must bypass environment proxies and must not follow redirects.
- Accept only public, global-unicast addresses and explicitly reject private, shared, documentation, benchmarking, reserved, transition, and other special-purpose ranges. Normalize addresses with `netip.Addr.Unmap` before comparing or storing them.
- Detect IPv6 and IPv4 independently. Synchronize both `AAAA` and `A` records when both families produce valid public addresses; if only one family succeeds, synchronize only that family.
- Keep per-service fallback behavior within each family. A detection or DNS synchronization failure for one family must not prevent attempting the other available family.
- Keep Yandex API calls behind `DNSProvider`; do not let provider-specific details leak into updater logic.

## Tests

- Keep unit tests fast, independent, repeatable, and free of external network calls.
- Use injected stubs or `httptest.Server` for HTTP behavior.
- Cover configuration compatibility, family-bound direct detection, redirect rejection, HTTPS/public and special-purpose address validation, per-service fallback, simultaneous IPv6/IPv4 synchronization, single-family fallback, canonical address comparison, timeout budgeting, and Yandex request behavior.
- Add a focused regression test with every behavior change.

## Required checks

Run these from this directory before handing off a change:

```sh
gofmt -w cmd/update-ip/*.go internal/updateip/*.go
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
```

Review the final diff after the checks. Keep unrelated workspace changes untouched.
