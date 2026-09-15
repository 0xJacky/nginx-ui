---
name: release
description: Use when preparing or publishing an NGINX UI release, including version bumping with version.sh, release note drafting, release-prep commits, Avira pre-publication checks, annotated tags, pushing dev and tags, and creating GitHub Releases with Announcements discussions.
---

# NGINX UI Release

Use this workflow for NGINX UI releases from the repository root.

## Preconditions

- Work from the `dev` branch.
- Inspect `git status --short --branch` before changing files.
- If the user asks to commit existing workspace changes first, inspect recent commit style and commit that work separately before release prep.
- Keep staging explicit. Do not include unrelated local changes.
- Treat `release-notes-vX.Y.Z.md` as a temporary local release artifact, not a committed file.

## Version Preparation

1. Run `./version.sh` outside the sandbox when possible. It updates `app/package.json`, runs the frontend build, and refreshes generated artifacts that can require network access.
2. Enter the release version as `vX.Y.Z` when prompted and confirm it.
3. Check the generated diff with `git status --short` and `git diff --stat`.
4. Commit only version-preparation artifacts with:

```bash
git add <version-prep-files>
git commit -m "chore: prepare vX.Y.Z"
```

Do not commit `release-notes-vX.Y.Z.md`.

## Release Notes

Create `release-notes-vX.Y.Z.md` in the repository root using exactly these sections:

```markdown
## Features

- <User-facing feature summary> by @<contributor> ([<short-hash>](https://github.com/0xJacky/nginx-ui/commit/<full-sha>))

## Bug Fixes

- <User-facing fix summary> by @<contributor> ([<short-hash>](https://github.com/0xJacky/nginx-ui/commit/<full-sha>))

## Contributors

@handle
```

Guidelines:

- Base the notes on the verified range from the previous release tag to `HEAD`.
- Every actual change bullet must include both the verified contributor's `@handle` and a linked short commit hash, even when a PR link is also included. The `Contributors` section does not replace per-change attribution.
- Resolve handles from the merged PR author and verified co-authors, or the GitHub-linked commit author for direct commits. Do not automatically credit the merger or committer, and never infer a handle from a display name or email. Resolve missing attribution before finalizing the notes; if it cannot be verified, ask the user for the handle or an explicit attribution exception.
- Use a commit that implements the summarized change and belongs to the verified release range. For squash merges, use the resulting squash commit; for rebased or cherry-picked changes, use the commit shipped in this repository. Never use the release-prep commit as a substitute.
- Display a unique abbreviated SHA, starting at 8 characters and extending it if ambiguous, and link it to the full commit SHA on GitHub.
- Write concise summaries of notable user-facing outcomes rather than copying the raw commit log. Group commits only when they describe one coherent change; retain all relevant contributor handles and the linked hashes needed to substantiate the summary. Split unrelated changes into separate bullets.
- List verified release contributors once each in `Contributors`, including everyone credited inline.
- If a change section is empty, use `- None.`; this sentinel needs no contributor or hash.
- Do not include test status unless the user explicitly asks for it.

Before finalizing, check every change bullet for accurate attribution, a resolving commit link, and membership in the release range. Reuse the same reviewed notes for the tag annotation and GitHub Release body.

References: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) for curated, grouped summaries and [GitHub release notes](https://docs.github.com/en/repositories/releasing-projects-on-github/automatically-generated-release-notes) for contributor credit and change traceability. Per-change handles and short hashes are this project's required format.

## Validation

- `./version.sh` already runs the frontend build and Go generation.
- By default, run lightweight checks such as `git diff --check`.
- Run broader tests only when appropriate for the release scope or when the user requests them. If the user says to skip tests, do not keep trying to run them.
- If local tests are affected by a parent Go workspace, use repo-isolated mode such as `GOWORK=off` and a writable `GOCACHE`.

## Avira Pre-Publication Gate

Complete this gate after the release-prep commit and before creating or pushing the release tag or publishing the GitHub Release:

1. Record the exact release-candidate commit from `git rev-parse HEAD`.
2. After explicit publication authorization, push only `dev` so the `Build` workflow can produce the Windows amd64 artifact from that exact commit. Do not push the release tag yet.
3. Wait for the `Build` workflow for the candidate commit to succeed. Download its Windows amd64 artifact and verify that it contains the expected `nginx-ui.exe`.
4. Record the executable SHA-256. Archive only `nginx-ui.exe` in a password-protected ZIP under Avira's 50 MB limit using password `infected`.
5. Immediately before transmitting the archive and contact details, obtain the required external-submission confirmation. Submit the sample to Avira VirusLab as `Suspected False Positive (Not Malware)` with the planned version, candidate commit, executable hash, source URL, and release URL.
6. Stop until Avira explicitly reports `Clean`. A pending result or detection leaves the release tag and GitHub Release unpublished.
7. Recheck that `HEAD` and `origin/dev` still resolve to the scanned candidate commit. Any commit change invalidates the result and requires a new artifact and scan.

The pre-publication artifact is content-level evidence, not guaranteed byte-for-byte proof of the final GitHub Release asset because the build embeds `settings.buildTime`. After publication, record the exact published Windows ZIP and extracted EXE SHA-256 values. If WinGet flags the final executable, submit that exact EXE to Avira before requesting an ESRP rerun.

## Tag, Push, And Publish

Only after the Avira pre-publication result is `Clean` and the candidate commit is unchanged:

```bash
git -c tag.gpgSign=false tag -a vX.Y.Z -F release-notes-vX.Y.Z.md
git push origin vX.Y.Z
gh release create vX.Y.Z --verify-tag --title vX.Y.Z -F release-notes-vX.Y.Z.md --discussion-category Announcements
```

Notes:

- Use `git -c tag.gpgSign=false tag -a ...` when local GPG signing blocks tag creation.
- The GitHub Release command is expected to create the matching Announcements discussion.
- Verify publication with `gh release view vX.Y.Z` and, if needed, inspect recent Discussions in the `Announcements` category.
- Download the final `nginx-ui-windows-64.zip`, record its SHA-256, extract `nginx-ui.exe`, and record the executable SHA-256.
- After a successful release, leave the release-note markdown untracked unless the user asks to delete it.
