# ADR 0014: Separate cognitive Profile and signed publication

## Status

Accepted. Supersedes ADR 0002.

## Context

AgentProfile must represent private cognition, while AgentFacts and AgentCard serve different external protocols. Treating all three as one public identity object would disclose fallible short-term observations, confuse trust with communication, and require an A2A endpoint that does not exist.

## Decision

- Keep Impression and Confirmed Fact as separate states; only the owner can promote a Fact Candidate.
- Keep AgentProfile private and inject bounded, labelled Profile context into Runtime.
- Enforce Disclosure Policy only over identity, capability, Confirmed Fact, and endpoint subjects. Impression is never externally disclosable.
- Sign versioned AgentFacts publications with per-Agent Ed25519 keys encrypted by an operator master key. Support hostname verification, expiry, revocation, and privacy-preserving POST queries.
- Use official A2A Go v2.4.0 types for an owner-only AgentCard Draft. Do not publish it until a real supported interface exists.
- Do not implement a central Index or claim third-party attestation.

## Consequences

The system gains a complete private-cognition-to-trusted-publication boundary without presenting self-signing as universal Agent identity. Operators must safely manage `AGENT_KEY_ENCRYPTION_KEY`; losing or changing it makes existing private keys unusable and requires rotation/republication.
