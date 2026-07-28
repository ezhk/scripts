# update-ip contributor guidance

## Scope

These instructions apply to this Go module and every file below this directory.

## Current project context

- This Go module replaces the legacy `../update-ip.py` script and targets Go 1.22. Its direct dependencies are Zap and `yaml.v3`.
- `cmd/update-ip/main.go` is the small executable entry point. `internal/updateip/app.go` loads configuration, creates IPv6-first detectors and the Yandex client, applies a 30-second run timeout, and invokes `Updater.Run`.
- `internal/updateip/config.go` owns the five-field YAML model and validation. `configs/update_ip.yaml` is the compatibility fixture and example.
- `internal/updateip/detector.go` owns address-family binding and HTTPS discovery. `HTTPSAddressDetector` tries configured services in order; detector errors trigger fallback.
- `internal/updateip/updater.go` owns provider-independent orchestration. It selects any valid public IPv6 result before the first public IPv4 fallback, then changes only that family record.
- `internal/updateip/yandex.go` implements `DNSProvider` through the Yandex 360 API. HTTP execution stays behind `HTTPDoer`; request construction and response validation remain separate.
- Tests use in-package stubs and `httptest.Server`; they must never call real address or DNS services.
- Runtime flow is: load and validate config; detect a public address; list records; skip an equal canonical address or add/update one record; emit structured Zap fields.

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

- Preserve the YAML keys `token`, `organization`, `domain`, `subdomain`, and `ttl`, and keep `/etc/update-ip.yml` as the default path.
- Parse configuration with the direct `yaml.Unmarshal` API. Do not introduce a generalized configuration layer for this five-field format.
- External address detection must use HTTPS, bound to the requested address family: `tcp6` with `::` for IPv6 and `tcp4` with `0.0.0.0` for IPv4.
- Accept only public, global-unicast addresses. Normalize addresses with `netip.Addr.Unmap` before comparing or storing them.
- Prefer a detected IPv6 address. Use IPv4 only when no detector returns a valid public IPv6 address.
- Keep address-service fallback behavior and do not update the other DNS record family during a run.
- Keep Yandex API calls behind `DNSProvider`; do not let provider-specific details leak into updater logic.

## Tests

- Keep unit tests fast, independent, repeatable, and free of external network calls.
- Use injected stubs or `httptest.Server` for HTTP behavior.
- Cover configuration compatibility, family-bound detection, HTTPS/public-address validation, service fallback, IPv6 preference, NAT-visible IPv4 fallback, canonical address comparison, and Yandex request behavior.
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
