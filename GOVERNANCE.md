# Governance

How Telark is run and how you move up the contributor ladder. Modeled on the
[CNCF contributor ladder](https://github.com/cncf/project-template/blob/main/CONTRIBUTOR_LADDER.md).
All participation is bound by the [Code of Conduct](CODE_OF_CONDUCT.md).

The thresholds below are the project's current, deliberately concrete values.
They can be changed by maintainer consensus (see *Changing this document*).

## Roles

### Contributor
Anyone who opens an issue, comments, reviews or sends a pull request. No approval
is needed; follow [CONTRIBUTING.md](CONTRIBUTING.md).

- **Rights:** open issues/PRs, comment, review (non-binding).
- **Expectations:** follow the Code of Conduct and the conventions in
  [CONVENTIONS.md](CONVENTIONS.md); keep PRs surgical and CI green.

### Reviewer
Trusted to give meaningful review on a defined area (a `CODEOWNERS` subtree, e.g.
`/services/auth` or `/charts`).

- **Entry criteria (all of):**
  - **≥ 8 merged, non-trivial PRs** to that area, over **≥ 2 months** of activity.
  - Demonstrated understanding of the area in review comments and PRs.
  - Nominated by a Maintainer via a PR adding them to `CODEOWNERS`.
- **Approval:** lazy consensus of Maintainers: merged if no Maintainer objects
  within **5 business days**.
- **Rights:** listed in `CODEOWNERS` for the area; their review counts toward the
  approval a PR needs. Cannot merge.
- **Kept by:** reviewing regularly; a review roughly every few weeks.

### Maintainer
Owns the project's direction and has merge + release authority.

- **Entry criteria (all of):**
  - Active **Reviewer for ≥ 3 months** with sustained, high-quality reviews and at
    least one substantial feature or subsystem shipped.
  - Nominated by an existing Maintainer.
  - Approved by a **majority of current Maintainers** (the nominee abstains).
- **Rights:** merge PRs, cut releases, approve promotions, vote on governance.
- **Responsibilities:** uphold conventions and CI gates, triage, mentor Reviewers,
  respond to security reports (see [SECURITY.md](SECURITY.md)), keep releases healthy.

### Emeritus
A former Maintainer who has stepped back.

- **Trigger:** self-request, or **6 months** of inactivity (no reviews/merges).
- **Effect:** write access removed; keeps recognition in the maintainer list under
  *Emeritus*. May return via the Maintainer criteria, fast-tracked at the discretion
  of active Maintainers.

## Decision making

- **Default: lazy consensus.** A change proceeds unless a Maintainer objects.
- **Code changes:** a PR merges when it has **≥ 1 Maintainer approval** (or the
  approval of a Reviewer who owns the touched `CODEOWNERS` area **plus** a Maintainer),
  CI is green, and no unresolved objection stands.
- **Contested changes / anything touching this file, the license, or release
  policy:** decided by **majority vote of Maintainers**; ties are declined
  (status quo wins).
- **Security issues:** handled privately per [SECURITY.md](SECURITY.md), never in a
  public issue.

## Adding / removing Maintainers

- **Add:** nomination + majority approval of current Maintainers, landed as a PR to
  this file and `CODEOWNERS`.
- **Remove / step down:** self-request at any time, or by majority vote for Code of
  Conduct violations or sustained inactivity → moved to *Emeritus*.

## Changing this document

Amendments are PRs to `GOVERNANCE.md` and require majority approval of Maintainers.

## Maintainers

| Name | GitHub | Areas |
|---|---|---|
| telark | [@hourki](https://github.com/hourki) | all (`CODEOWNERS`), founding maintainer |

*Emeritus:* none.
