# ci/

The scripts the CI workflows run. `rust/run.sh` is the Rust gate:
`.github/workflows/rust.yml` runs it, and so can you.

To change CI, edit `.github/workflows/` in a reviewed pull request.
Session credentials push workflow files (admin `DECISIONS.md` ADR-8, as
amended 2026-09-24), so staging a workflow here first for a maintainer
to promote is optional. Sessions still cannot push tags, so a maintainer
pushes any tag that a tag-triggered workflow needs.

Some of the workflows are maintained in admin as well, and an edit made
only in this repository does not last. A workflow with a template in
admin `rollout/workflows/`, named `railroad__<file>`, changes in that
template too, in a pull request to admin. Today that is `ci.yml`,
`release.yml`, `crates-release.yml`, `github-release.yml`,
`notify-status.yml` and `scorecard.yml`. Admin `scripts/verify.sh`
reports a deployed copy that differs from its template, and the next
`rollout/apply-workflows.sh --apply` writes the template back over it.

## Promoted

Both workflows staged here have been promoted and now live in
`.github/workflows/`:

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
