# Releases

[VERSION](../../VERSION) is the version source. Increment the patch for fixes and the minor for features; while the app is `0.x`, breaking changes also increment the minor. Record changes in [CHANGELOG.md](../../CHANGELOG.md).

Releases are built and published by GitHub Actions from a signed tag. Nothing is uploaded from a maintainer's machine.

## Publishing a release

1. On a branch, set `VERSION` to the new version and rename the `## Unreleased`
   heading in `CHANGELOG.md` to `## <version> — <date>`, for example
   `## 0.4.0 — 2026-11-20`. That section becomes the release notes.
2. Open a pull request and merge it once CI passes.
3. Tag the merged commit on `main` with a signed tag and push the tag:

   ```sh
   git switch main && git pull --ff-only
   git tag -s "v$(cat VERSION)" -m "Shellyficator $(cat VERSION)"
   git push origin "v$(cat VERSION)"
   ```

4. Follow the **Release** workflow under the repository's Actions tab. When it
   finishes, the release appears under Releases and the image on GHCR.

Add a new `## Unreleased` section to the changelog when work continues.

For a prerelease, use a version such as `0.5.0-rc.1` in `VERSION`, the changelog
heading, and the tag. GitHub marks it as a prerelease, and the image gets only
its exact version tag.

## What the release workflow checks and builds

The [release workflow](../../.github/workflows/release.yml) runs on tags
matching `vX.Y.Z`:

1. **Verify:** the tag name is valid semver; the tag is annotated and signed by a
   key in [`.github/allowed_signers`](../../.github/allowed_signers); `VERSION`
   matches the tag; the commit is on `main`; `CHANGELOG.md` has a section for the
   version; and the `ci-ok` check passed for the commit.
2. **Binaries:** [GoReleaser](../../.goreleaser.yaml) builds Linux `amd64` and
   `arm64` archives, `checksums.txt`, and SBOMs, then creates a draft release
   with the changelog section as notes. The checksums are attested, and the
   attestation is attached to the release as a Sigstore bundle.
3. **Image:** builds the [Dockerfile](../../Dockerfile) for `linux/amd64` and
   `linux/arm64`, pushes it to GHCR, and attests the image digest.
4. **Publish:** publishes the draft. Immutable releases are enabled, so from this
   point the tag and assets cannot be changed or deleted.

Attestations are signed with Sigstore through GitHub and record which workflow,
commit, and tag produced each artifact.

## Container image tags

Images are published to `ghcr.io/dkalinkov/shellyficator`.

| Tag | Meaning |
| --- | --- |
| `0.4.0` | One release. Never moves. Use this in deployments. |
| `0.4` | Newest patch release of that minor version. |
| `1` | Newest release of that major version, from 1.0.0 on. |
| `latest` | Newest stable release. |
| `edge` | Latest `main` after CI passed. A development build that may be unstable. |

## When a release fails

- **Before the publish step:** the tag and a draft release exist. If the failure
  was transient, re-run the failed jobs; GoReleaser replaces the draft.
- **If the tagged commit is wrong:** release tags cannot be moved or deleted.
  Delete the draft release, fix the problem, and release the next patch version.
- **After publishing:** the release is immutable. Publish a new patch release.

## Verifying a release

```sh
# The release, its tag, and assets are unchanged since publication.
gh release verify v0.4.0 --repo dkalinkov/shellyficator

# An archive or the image was built by this repository's release workflow.
gh attestation verify shellyficator_0.4.0_linux_amd64.tar.gz --repo dkalinkov/shellyficator
gh attestation verify oci://ghcr.io/dkalinkov/shellyficator:0.4.0 --repo dkalinkov/shellyficator

# The same check using the bundle attached to the release (0.4.1 and later)
# instead of GitHub's attestation API.
gh attestation verify shellyficator_0.4.1_linux_amd64.tar.gz \
  --bundle shellyficator_0.4.1_provenance.sigstore.json --repo dkalinkov/shellyficator

# Checksums of downloaded archives.
sha256sum --check --ignore-missing checksums.txt

# The tag signature, from a clone of the repository.
git -c gpg.ssh.allowedSignersFile=.github/allowed_signers verify-tag v0.4.0
```

## Local builds

`make build` and `make docker` report the base version for a clean matching tag, otherwise append `-dev`. `--version` also reports the commit, dirty marker, and UTC build time; the UI shows the app version. `make dev`, plain Go builds, and Docker builds without metadata report a development version. Local Docker images use `shellyficator:local` and `shellyficator:<app-version>` tags. Run `goreleaser release --snapshot --clean` to try the release build without publishing.

Database schema changes use separate dated Go migrations. Preserve app data across upgrades.

See [deployment and backups](deployment.md) before upgrading an installation.

## Maintainer setup

Repository settings that protect releases:

- **`main` ruleset:** changes arrive through pull requests with a passing `ci-ok`
  check; force pushes and deletion are blocked. Repository admins can bypass it.
- **Release tag ruleset:** only repository admins can create `v*` tags, and
  nobody can move or delete them.
- **Immutable releases** are enabled, and Actions must be pinned to full commit SHAs.

Release tags are signed with SSH. In a clone, configure signing once:

```sh
git config gpg.format ssh
git config user.signingkey ~/.ssh/<key>.pub
git config tag.gpgSign true
git config gpg.ssh.allowedSignersFile "$PWD/.github/allowed_signers"
```

Add the public key to `.github/allowed_signers` and, so GitHub shows tags as
verified, to your GitHub account as a **signing key** (Settings → SSH and GPG keys).
