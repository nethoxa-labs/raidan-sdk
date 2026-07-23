# Repository instructions

Read `/Users/nethoxa/Desktop/AGENTS.md`, `/Users/nethoxa/Desktop/ARCHITECTURE.md`,
and every applicable canonical contract in `raidan-specs/docs/` before making
changes.

This repository owns the stable reusable Ethereum protocol, session, target,
result, and wire primitives consumed by network workers. Keep worker runtime,
catalog, client orchestration, deployment commands, Docker files, and release
helpers in `raidan-worker-ethereum`; keep network-neutral execution in
`raidan-core`. Existing SDK tags and published releases are immutable: never
rewrite, modify, or republish them. Do not add compatibility paths, duplicate
worker implementations, tests, or speculative abstractions. Commit each new
component as `<type>(<component>): <description summary>` and preserve all
existing commits.
