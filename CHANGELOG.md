# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-08-25

### Changed
- Default binary name changed from `umeshd` to `umeshnode` (`-binary-name` / `BINARY_NAME` env).
- Default node home directory changed from `.umeshd` to `.umeshnode` (`-node-dir` / `NODE_DIR` env).
- Derived BaseApp name updated from `UmeshApp` to `UmeshnodeApp`.
- Documentation, tests, and examples updated to use `umeshnode`.

### Migration
- If you were explicitly passing `-binary-name umeshd` or `-node-dir .umeshd`, update to `umeshnode` / `.umeshnode`.
- Generated trees from earlier versions will use the old identifiers; regenerate with new defaults if needed.

### CI
- Upgraded GitHub Actions runners from `ubuntu-24.04` to `ubuntu-26.04`.
- Optimized release assets: sign and checksum only the release archive.

## [0.1.0] - 2026-08-24

### Added
- Initial release of `umeshprep` — a Go build tool that generates the Umesh source tree from CosmWasm `wasmd`.
- AST-based patching for Cosmos SDK v0.54 + CometBFT v0.39 compatibility.
- Module toggling (IBC, upgrade, feegrant, authz, vesting, protocolpool, etc.).
- Capabilities filtering and `EndBlockers` reordering (bank first).
- Automated CI/CD: `vet`, `test`, `build`, `lint` on push/PR; release workflow with GPG signing + SHA-256 checksums on `v*` tags.
- Apache-2.0 License.
