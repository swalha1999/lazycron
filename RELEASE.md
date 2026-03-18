# Release Guidelines

## Prerequisites

Set up these GitHub repository secrets (Settings > Secrets and variables > Actions):

| Secret | Purpose |
|---|---|
| `GITHUB_TOKEN` | Provided automatically by GitHub Actions |
| `AUR_SSH_KEY` | SSH private key for publishing to AUR |
| `HOMEBREW_TAP_TOKEN` | A GitHub PAT with `repo` scope for pushing to `swalha1999/homebrew-tap` |

## How to Release

1. Make sure you're on `main` with a clean working tree:

   ```sh
   git checkout main
   git pull
   git status
   ```

2. Tag the release with a semantic version:

   ```sh
   git tag v0.1.0
   ```

3. Push the tag:

   ```sh
   git push origin v0.1.0
   ```

That's it. GitHub Actions will automatically:

- Build binaries for **linux** and **darwin** (amd64 + arm64)
- Create a **GitHub Release** with tar.gz archives and checksums
- Push **deb**, **rpm**, and **archlinux** packages
- Update the **Homebrew tap** (`swalha1999/homebrew-tap`)
- Publish to the **AUR** (`lazycron-bin`)

## How Users Install

| Platform | Command |
|---|---|
| macOS / Linux (Homebrew) | `brew install swalha1999/tap/lazycron` |
| Arch Linux (AUR) | `yay -S lazycron-bin` |
| Debian / Ubuntu | Download `.deb` from GitHub Releases |
| RHEL / Fedora | Download `.rpm` from GitHub Releases |
| Any POSIX | Download the binary from GitHub Releases |

## Adding FreeBSD / Other POSIX Systems

Add `freebsd` to the `goos` list in `.goreleaser.yml`:

```yaml
goos:
  - linux
  - darwin
  - freebsd
```
