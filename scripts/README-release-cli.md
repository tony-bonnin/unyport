# Release CLI

This repository includes a helper script to publish the same release notes to GitHub, GitLab, and Codeberg.
It updates the hardcoded Go app version from the tag in both backends, creates the release commit, creates the tag, pushes branch and tag, then creates or updates the releases.

## Expected token files

- `/root/.config/gh/unyport-token`
- `/root/.config/gl/unyport-token`
- `/root/.config/cb/unyport-token`

## Usage

```sh
chmod +x scripts/release-all.sh
sh ./scripts/release-all.sh v0.1.0
```

The script defaults to `RELEASE-v0.1.0.md` when called with `v0.1.0`.
It updates `unyport/backend/config/version.go` and `../docker_demo/unyport/backend/config/version.go`, commits the release change, creates the tag if needed, pushes `master`, pushes the tag to `origin`, then creates or updates the matching releases.
If a release already exists, it is updated instead of failing.
If Codeberg releases are disabled for the repository, the script skips Codeberg cleanly.

If you need to replace an existing remote tag:

```sh
FORCE_TAG=1 sh ./scripts/release-all.sh v0.1.0
```
