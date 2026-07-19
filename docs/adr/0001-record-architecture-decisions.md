# 1. Record architecture decisions

Date: 2026-07-19

## Status

Accepted

## Context

We need a durable record of significant architectural decisions — why they were made, what was considered, and what follows — so future contributors do not re-litigate settled ground or reverse a decision without seeing its rationale.

## Decision

We use Architecture Decision Records, one Markdown file per decision under `docs/adr/`, numbered sequentially, in the format described by Michael Nygard: **Status**, **Context**, **Decision**, **Consequences**. A decision is superseded, not deleted; the superseding ADR links back.

## Consequences

- Every non-obvious architectural choice gets a short, reviewable record alongside the code.
- The ADR log doubles as onboarding material.
- Small decisions do not need an ADR; use judgement.
