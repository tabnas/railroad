# ci/

Staging area for GitHub Actions workflow changes.

This directory exists because session credentials cannot write
`.github/workflows/*` — see admin `DECISIONS.md` ADR-8. To change CI:

1. Put the intended workflow file in `workflows/`.
2. A maintainer promotes it with the admin `rollout/apply-ci-folders.sh`
   script.

## Promoted

Nothing is pending. Both workflows staged here have been promoted and now
live in `.github/workflows/`:

- **`rust.yml`**, the Rust gate. It checks this repository out beside
  fresh clones of `parser`, `json` and `support` (the crate's three
  unpublished path dependencies), installs the MSRV pinned in
  `rs/Cargo.toml`, and runs `rust/run.sh`: format check, build, tests,
  doctests, clippy with warnings denied, and a lockfile check that exempts
  only the siblings' own versions. The script is the whole gate, so a local
  `ci/rust/run.sh` says what CI would.
- **`docs.yml`**, the prose gate: Vale over the reader-facing pages at the
  levels set in `.vale.ini`, on the file list `ts/scripts/gated-docs.cjs`
  produces. See `docs/STYLE-GUIDE.md`. `make prose` runs the identical
  check locally.
