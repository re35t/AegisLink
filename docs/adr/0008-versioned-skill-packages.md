# ADR 0008: Versioned Skill Packages and Agent Bindings

Status: Accepted and implemented.

Date: 2026-08-24.

## Decision

A Principal owns stable `skill_packages`. Each package contains immutable `skill_versions`, and `agent_skills` stores one selected version plus enablement per Agent.

Inline installs without a version label use `local-<hash prefix>`. Reinstalling identical content reuses its version. Reusing an explicit label with different content is rejected. Installing another version only switches the target Agent, so other Agents retain their selected versions.

Uninstall removes only the Agent binding; packages and versions remain available for reuse and audit. Runtime resolution continues to load only the enabled version selected by the authoritative Conversation Agent.

Migration `00004_skill_packages_and_versions.sql` converts every previous Skill into one package, one version, and its existing bindings without losing content, hashes, or enablement.

## Evolution

ADR 0009 adds local Markdown/ZIP bundles and immutable files on top of this version model. Package libraries, historical version selection, Git provenance, signatures, executable scripts, and physical package purge remain future work.
