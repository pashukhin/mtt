# Releasing

mtt ships as prebuilt binaries attached to a GitHub Release, built by
[.github/workflows/release.yml](.github/workflows/release.yml) on a `v*` tag. The released binaries are
version-stamped from the **tag**, not from any committed string.

## Cutting a release

1. **Pick the bump.** The version *is* the git tag — no source edit. Choose `X.Y.Z` from the accumulated
   `[Unreleased]` CHANGELOG categories per the **Versioning policy** below. Unreleased builds report a
   `git describe` string; `go install …@vX.Y.Z` reports the module version.
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

## Versioning policy

mtt follows [Semantic Versioning](https://semver.org). The annotated git tag `vX.Y.Z` is the single source
of truth; the version is derived at build time (ldflags / `git describe`) and at run time from the module
build info — nothing is hand-maintained in source.

> Stripping is safe: `-ldflags "-s -w"` (or `strip`) removes the symbol table and DWARF, **not** the
> `-X`-injected version nor the allocated `.go.buildinfo` section that `runtime/debug.ReadBuildInfo` reads —
> a stripped release binary still reports its version.

**Pre-1.0 (0.y.z):** bump **MINOR** (`0.→Y←.0`) for any new feature and/or any backward-incompatible change;
bump **PATCH** (`0.y.→Z←`) for backward-compatible fixes, security fixes, docs, and internal changes. A
breaking change is never shipped as a PATCH — it forces a MINOR and a `Changed`/`Removed` CHANGELOG entry
with a migration note.

**The compat surface** SemVer governs: the CLI (commands, flags, positional grammar, exit codes, and the
`MTT_DIR`/`MTT_BY`/`MTT_ROLE` env vars), the `--json` output schema, and the `.mtt` store schema *and its
semantics* (the `{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}` placeholder vocabulary, the exit-code/gate-block
contract, the `require:` keys). The public `pkg/mtt` Go API is best-effort pre-1.0.

**Bump from the changelog (at release cut):** any `Added`/`Changed`/`Deprecated`/`Removed` in `[Unreleased]`
→ MINOR; only `Fixed`/`Security`/docs/internal → PATCH.
