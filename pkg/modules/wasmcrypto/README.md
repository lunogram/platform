# wasmcrypto

This package contains minimal cryptographic helpers used by TinyGo-compiled WASM modules.

## Why this exists

With TinyGo `0.40.1` and Go `1.25.x`, we observed runtime panics in WASM providers when using standard library signing paths for RS256 JWTs (`crypto/rsa` with `crypto.SHA256`). This is tracked upstream in [tinygo-org/tinygo#5291](https://github.com/tinygo-org/tinygo/issues/5291).

The panic manifests as:

- `wasm error: unreachable`
- `main.runtime.nilPanic()`
- often during provider `send()` while building OAuth/JWT auth tokens.

Downgrading to Go `1.23.x` avoids it, but we prefer to stay on current Go.

## Scope

This package is intentionally small and only contains what providers need right now:

- `Sum256`: SHA-256 implementation without relying on `crypto/sha256`
- `SignRS256PKCS1v15`: RS256 PKCS#1 v1.5 signing helper for pre-hashed input
- `SignES256P256`: ES256 (ECDSA P-256) signing helper returning compact JWS signature form
- `HMACSHA256`: HMAC-SHA256 helper without relying on `crypto/hmac`

These helpers are used as a compatibility shim for TinyGo WASM builds.

## Security notes

- The implementation mirrors standard SHA-256 and PKCS#1 v1.5 encoding/signing behavior used for JWT RS256.
- Keep this package narrow in scope and avoid adding unrelated crypto primitives.
- Prefer standard library crypto paths once TinyGo/runtime compatibility is fixed upstream.

## Removal plan

When TinyGo fully supports the affected Go `1.25+` crypto paths for our WASM targets, migrate providers back to standard library usage and remove this package.
