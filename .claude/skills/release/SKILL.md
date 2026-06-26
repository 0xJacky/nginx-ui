---
name: release
description: Use when preparing or publishing an NGINX UI release, including version bumping with version.sh, release note drafting, release-prep commits, annotated tags, pushing dev and tags, and creating GitHub Releases with Announcements discussions.
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

- ...

## Bug Fixes

- ...

## Contributors

@handle
```

Guidelines:

- Base the notes on the verified range from the previous release tag to `HEAD`.
- Prefer GitHub handles for contributors when known from merged PRs or commit metadata.
- Use commit SHAs when PR numbers are not needed or not available.
- If there are no feature entries, use `- None.` under `Features`.
- Do not include test status unless the user explicitly asks for it.

## Validation

- `./version.sh` already runs the frontend build and Go generation.
- By default, run lightweight checks such as `git diff --check`.
- Run broader tests only when appropriate for the release scope or when the user requests them. If the user says to skip tests, do not keep trying to run them.
- If local tests are affected by a parent Go workspace, use repo-isolated mode such as `GOWORK=off` and a writable `GOCACHE`.

## Tag, Push, And Publish

After the release-prep commit is created:

```bash
git -c tag.gpgSign=false tag -a vX.Y.Z -F release-notes-vX.Y.Z.md
git push origin dev vX.Y.Z
gh release create vX.Y.Z --verify-tag --title vX.Y.Z -F release-notes-vX.Y.Z.md --discussion-category Announcements
```

Notes:

- Use `git -c tag.gpgSign=false tag -a ...` when local GPG signing blocks tag creation.
- The GitHub Release command is expected to create the matching Announcements discussion.
- Verify publication with `gh release view vX.Y.Z` and, if needed, inspect recent Discussions in the `Announcements` category.
- After a successful release, leave the release-note markdown untracked unless the user asks to delete it.
