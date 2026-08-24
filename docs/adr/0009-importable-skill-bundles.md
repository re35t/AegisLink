# ADR 0009: Importable Skill Bundles and Read-only Resources

Status: accepted and implemented.

Date: 2026-08-24.

## Decision

- Inline authoring continues to install a JSON REST `SKILL.md` request.
- Local import uses multipart REST and accepts one Markdown file or ZIP bundle.
- A ZIP may have one common top-level directory. After normalization it must contain root `SKILL.md`; other files are limited to `references/`, `assets/`, or `scripts/`.
- Normalized immutable files are stored in PostgreSQL `skill_version_files` with path, media type, size, hash, and text-readable metadata.
- The bundle hash is deterministic over sorted paths and content. An immutable Version is reused only for identical content.
- Runtime `load_skill` returns `SKILL.md` plus resource metadata. `read_skill_resource` reads UTF-8 text only from the Version enabled for the current Agent.
- Files under `scripts/` are stored and may be read, but receive no execution authority. Binary assets are not inserted into model text context.

## Security limits

- Uploads are limited to 4 MiB; expanded bundles to 8 MiB, 64 files, and 1 MiB per file.
- Absolute paths, traversal, backslashes, NUL, symlinks, duplicate normalized paths, and undeclared root files are rejected.
- `SKILL.md` retains the existing frontmatter, UTF-8, and 128 KiB limits.
- Runtime reads only UTF-8 resources up to 256 KiB. Import is not trust, signing, or execution approval.

## Product workflow

- Create or customize authors a new Skill or copies the selected Version into the editor to create another Version.
- Import file uploads `.md` or `.zip`, validates it, and binds the resulting Version to the current Agent.
- Remove from agent deletes only the binding; Package, Version, and files remain auditable and reusable.

Git sources, provenance, signatures, malware scanning, historical Version selection, bundle export, and controlled script execution remain future work.
