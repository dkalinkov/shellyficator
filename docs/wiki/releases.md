# Releases

Update [VERSION](../../VERSION) and [CHANGELOG.md](../../CHANGELOG.md). Use patch
versions for fixes and minor versions for features or breaking changes while at
`0.x`.

## Publishing a release

1. Set `VERSION` and rename `## Unreleased` to `## <version> — <date>` in
   `CHANGELOG.md`. That section becomes the release notes.
2. Merge the PR once CI passes. Sign and push the tag from `main`.

   ```sh
   git switch main && git pull --ff-only
   git tag -s "v$(cat VERSION)" -m "Shellyficator $(cat VERSION)"
   git push origin "v$(cat VERSION)"
   ```

3. After the **Release** workflow publishes, update the Home Assistant app
   in a [separate PR](home-assistant.md#maintaining-the-app).

Use a version like `0.5.0-rc.1` in all three places for prereleases. Their images
get only the exact tag. Start a new `## Unreleased` section for subsequent work.

## What the release workflow checks and builds

The [workflow](../../.github/workflows/release.yml) verifies the signed tag,
`VERSION`, changelog entry, ancestry on `main`, and passing `ci-ok` check.
[GoReleaser](../../.goreleaser.yaml) drafts the release with Linux amd64/arm64
archives, checksums, and SBOMs. CI builds and attests both container images and
the release archives before publishing the immutable release.

## Container image tags

| Tag | Meaning |
| --- | --- |
| `0.4.0` | Exact release. Use for deployments. |
| `0.4` | Latest patch of that minor version |
| `1` | Latest release of that major version, starting with 1.0.0 |
| `latest` | Latest stable release |
| `edge` | Latest `main` build after CI passed |

`ghcr.io/dkalinkov/shellyficator-ha` gets only exact tags because Home Assistant
pulls the configured version. Use `ghcr.io/dkalinkov/shellyficator` standalone.

## When a release fails

- Rerun failed jobs for a transient error before publishing. GoReleaser replaces the draft.
- If the tag points to the wrong commit, delete the draft and release a new patch.
- If a release is already published, release a new patch.

## Verifying a release

```sh
# The release, its tag, and assets are unchanged since publication.
gh release verify v0.4.0 --repo dkalinkov/shellyficator

# An archive or the image was built by this repository's release workflow.
gh attestation verify shellyficator_0.4.0_linux_amd64.tar.gz --repo dkalinkov/shellyficator
gh attestation verify oci://ghcr.io/dkalinkov/shellyficator:0.4.0 --repo dkalinkov/shellyficator

# The same check using the bundle attached to the release (0.5.0 and later)
# instead of GitHub's attestation API.
gh attestation verify shellyficator_0.5.0_linux_amd64.tar.gz \
  --bundle shellyficator_0.5.0_provenance.sigstore.json --repo dkalinkov/shellyficator

# Checksums of downloaded archives.
sha256sum --check --ignore-missing checksums.txt

# The tag signature, from a clone of the repository.
git -c gpg.ssh.allowedSignersFile=.github/allowed_signers verify-tag v0.4.0
```

## Local builds

`make build` and `make docker` use the base version on an exact clean tag and
append `-dev` otherwise. `--version` includes available commit and build metadata.
Run `goreleaser release --snapshot --clean` for a local release build.

## Maintainer setup

Protect `main` with PRs and passing `ci-ok`. Block branch and release-tag
deletion and force updates. Only admins can create release tags. Enable
immutable releases and pin Actions to full SHAs.

Configure SSH signing once per clone.

```sh
git config gpg.format ssh
git config user.signingkey ~/.ssh/<key>.pub
git config tag.gpgSign true
git config gpg.ssh.allowedSignersFile "$PWD/.github/allowed_signers"
```

Add the public key to `.github/allowed_signers` and, so GitHub shows tags as
verified, to your GitHub account as a **signing key** (Settings → SSH and GPG keys).
