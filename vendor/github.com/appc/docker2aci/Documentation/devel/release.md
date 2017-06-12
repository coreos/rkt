# docker2aci release guide

How to perform a release of docker2aci.
This guide is probably unnecessarily verbose, so improvements welcomed.
Only parts of the procedure are automated; this is somewhat intentional (manual steps for sanity checking) but it can probably be further scripted, please help.

The following example assumes we're going from version 0.9.0 (`v0.9.0`) to 0.9.1 (`v0.9.1`).

Let's get started:

- Start at the relevant milestone on GitHub (e.g. https://github.com/appc/docker2aci/milestones/v0.9.1): ensure all referenced issues are closed (or moved elsewhere, if they're not done). Close the milestone.
- Branch from the latest master, make sure your git status is clean
- Ensure the build is clean!
  - `git clean -ffdx && ./build.sh && ./tests/test.sh` should work
  - Integration tests on CI should be green
- Update the [release notes](https://github.com/appc/docker2aci/blob/master/CHANGELOG.md).
  Try to capture most of the salient changes since the last release, but don't go into unnecessary detail (better to link/reference the documentation wherever possible).

The docker2aci version is [hardcoded in the repository](https://github.com/appc/docker2aci/blob/master/lib/version.go#L19), so the first thing to do is bump it:

- Run `scripts/bump-release v0.9.1`.
  This should generate two commits: a bump to the actual release (e.g. v0.9.1), and then a bump to the release+git (e.g. v0.9.1+git).
  The actual release version should only exist in a single commit!
- Sanity check what the script did with `git diff HEAD^^` or similar.
- If the script didn't work, yell at the author and/or fix it.
  It can almost certainly be improved.
- File a PR and get a review from another [MAINTAINER](https://github.com/appc/docker2aci/blob/master/MAINTAINERS).
  This is useful to a) sanity check the diff, and b) be very explicit/public that a release is happening
- Ensure the CI on the release PR is green!

After merging and going back to master branch, we check out the release version and tag it:

- `git checkout HEAD^` should work; sanity check lib/version.go (the `Version` variable) after doing this
- Add a signed tag: `git tag -s v0.9.1`.
- Build docker2aci
  - `sudo git clean -ffdx && ./build.sh`
  - Sanity check `bin/docker2aci -version`
- Push the tag to GitHub: `git push --tags`

Now we switch to the GitHub web UI to conduct the release:

- https://github.com/appc/docker2aci/releases/new
- For now, check "This is a pre-release"
- Tag "v0.9.1", release title "v0.9.1"
- Copy-paste the release notes you added earlier in [CHANGELOG.md](https://github.com/appc/docker2aci/blob/master/CHANGELOG.md)
- You can also add a little more detail and polish to the release notes here if you wish, as it is more targeted towards users (vs the changelog being more for developers); use your best judgement and see previous releases on GH for examples.
- Attach the release.
  This is a simple tarball:

```
	export NAME="docker2aci-v0.9.1"
	mkdir $NAME
	cp bin/docker2aci $NAME/
	sudo chown -R root:root $NAME/
	tar czvf $NAME.tar.gz --numeric-owner $NAME/
```

- Attach the release signature; your personal GPG is okay for now:

```
	gpg --detach-sign $NAME.tar.gz
```

- Publish the release!

- Clean your git tree: `sudo git clean -ffdx`.
