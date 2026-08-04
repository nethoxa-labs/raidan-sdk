# Repository instructions

Read `/Users/nethoxa/Desktop/AGENTS.md`, `/Users/nethoxa/Desktop/ARCHITECTURE.md`,
and each applicable contract in `/Users/nethoxa/Desktop/internal/specifications/`
before you make changes.

This repository owns the stable reusable Ethereum protocol, session, target,
result, and wire primitives consumed by network workers. Keep worker runtime,
catalog, client orchestration, deployment commands, Docker files, and release
helpers in `raidan-worker-ethereum`; keep network-neutral execution in
`raidan-core`. Existing SDK tags and published releases are immutable: never
rewrite, modify, or republish them. Do not add compatibility paths, duplicate
worker implementations, tests, or speculative abstractions. Commit each new
component as `<type>(<component>): <description summary>` and preserve all
existing commits.

Write your answers in ASD-STE100 Simplified Technical English. ASD-STE100 Simplified Technical English is a controlled writing standard. Aerospace and defense groups made it. It helps people write clear technical text.
Key rules:
**Use approved words only.** The standard gives a word list. Each word has one meaning.
**Use one word for one idea.** Do not use two words for the same thing.
**Write short sentences.** Use 20 words or less for instructions.
**Use active voice.** Write "Turn the switch", not "The switch must be turned".
**Write short paragraphs.** Keep one topic in each paragraph.
