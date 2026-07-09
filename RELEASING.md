# Releasing

mtt ships as prebuilt binaries attached to a GitHub Release, built by
[.github/workflows/release.yml](.github/workflows/release.yml) on a `v*` tag. The released binaries are
version-stamped from the **tag**, not from any committed string.

## Cutting a release

1. **Bump the dev version.** Set `version` in [internal/cli/root.go](internal/cli/root.go) to the next
   `-dev` value if it hasn't been already (e.g. `0.9.0-dev`). This is what unreleased builds report.
2. **Green gate.** `make check` must pass on an up-to-date `main`.
3. **Changelog.** Move the `[Unreleased]` entries in [CHANGELOG.md](CHANGELOG.md) under a new `[X.Y.Z]`
   heading (dated).
4. **Tag & push.**
   ```sh
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```
5. **Publish (automatic).** `release.yml` runs `make release VERSION=vX.Y.Z` (cross-compiles the 5-platform
   matrix into `dist/` + `SHA256SUMS`) and `gh release create` attaches the assets with generated notes.
6. **Verify.** Download an asset and `SHA256SUMS`, run `sha256sum -c` on the asset's line, and confirm
   `mtt version` prints `vX.Y.Z`.

## Building binaries locally (no publish)

```sh
make release VERSION=vX.Y.Z   # -> dist/mtt_vX.Y.Z_<os>_<arch>[.exe] + dist/SHA256SUMS
```

Pre-1.0 versions mirror the session (see [sessions/README.md](sessions/README.md)): a full session bumps the
minor, a point-session the patch.
