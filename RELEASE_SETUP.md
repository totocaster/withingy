# Release Setup Instructions

This mirrors the `stamp` pipeline so we can tag releases confidently.

## ✅ Files Added Here

1. **`.goreleaser.yml`** – builds macOS/Linux (amd64+arm64), adds ldflags for version info, and updates the Homebrew tap.
2. **`.github/workflows/release.yml`** – GitHub Actions workflow that runs on `v*` tags, runs tests, and invokes GoReleaser.

## 📋 One-Time GitHub Tasks

1. **Repository visibility** – `https://github.com/totocaster/withingy` must be public before Homebrew installs can work for other people.
2. **Homebrew tap** – ensure `https://github.com/totocaster/homebrew-tap` exists (public) with a `Formula/` dir. A bootstrap `Formula/withingy.rb` can live there before the first release; GoReleaser will replace it with versioned release assets afterward.
3. **Secrets** – `HOMEBREW_TAP_TOKEN` is already set in repo secrets (thanks!). Nothing else required for release.

## 🚀 Cut a Release (example: v0.2.0)

```bash
# ensure tree is clean
git status

# update docs/changelog as needed, commit, push

# tag and push
git tag -a v0.2.0 -m "withingy v0.2.0"
git push origin v0.2.0
```

The release workflow runs automatically. Watch it under **Actions → Release**.

## ✅ After Workflow Completes

1. Check https://github.com/totocaster/withingy/releases for uploaded archives + checksums.
2. Verify the tap repo gained an updated `Formula/withingy.rb`.
3. Test install:
   ```bash
   brew tap totocaster/tap
   brew install withingy
   withingy --version
   ```

## 🔁 Future Releases

Same as above: merge your work, tag `v0.x.y`, push the tag, watch GoReleaser do the rest.

## 📚 References

- GoReleaser docs: https://goreleaser.com
- Stamp release playbook: `~/Developer/stamp/RELEASE_SETUP.md`
