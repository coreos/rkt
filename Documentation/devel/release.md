# rkt release guide

How to perform a release of rkt.
This guide is probably unnecessarily verbose, so improvements welcomed.
Only parts of the procedure are automated; this is somewhat intentional (manual steps for sanity checking) but it can probably be further scripted, please help.

The following example assumes we're going from version 0.6.1 (`v0.6.1`) to 0.7.0 (`v0.7.0`).

Let's get started:

- Start at the relevant milestone on GitHub (e.g. https://github.com/coreos/rkt/milestones/v0.7.0): ensure all referenced issues are closed (or moved elsewhere, if they're not done). Close the milestone.
- Update the [roadmap](https://github.com/coreos/rkt/blob/master/ROADMAP.md) to remove the release you're performing, if necessary
- Branch from the latest master, make sure your git status is clean
- Ensure the build is clean!
  - `git clean -ffdx && ./autogen.sh && ./configure --enable-functional-tests && make && make check` should work
  - Integration tests on CI should be green
- Update the [release notes](https://github.com/coreos/rkt/blob/master/CHANGELOG.md). Try to capture most of the salient changes since the last release, but don't go into unnecessary detail (better to link/reference the documentation wherever possible).

The rkt version is [hardcoded in the repository](https://github.com/coreos/rkt/blob/master/configure.ac#L2), so the first thing to do is bump it:
- Run `scripts/bump-release v0.7.0`. This should generate two commits: a bump to the actual release (e.g. v0.7.0), and then a bump to the release+git (e.g. v0.7.0+git). The actual release version should only exist in a single commit!
- Sanity check what the script did with `git diff HEAD^^` or similar. As well as changing the actual version, it also attempts to fix a bunch of references in the documentation etc.
- Fix the commit `HEAD^^` so that the version in `stage1/aci/aci-manifest.in` is correct.
- If the script didn't work, yell at the author and/or fix it. It can almost certainly be improved.
- File a PR and get a review from another [MAINTAINER](https://github.com/coreos/rkt/blob/master/MAINTAINERS). This is useful to a) sanity check the diff, and b) be very explicit/public that a release is happening
- Ensure the CI on the release PR is green!

After merging and going back to master branch, we check out the release version and tag it:
- `git checkout HEAD^` should work; sanity check configure.ac (2nd line) after doing this
- Build rkt, we'll use this in a minute:
  - `git clean -ffdx && ./autogen.sh && ./configure && make BUILDDIR=release-build`
    - This will build both `coreos` and `kvm` flavors and make `coreos` the default
    - Use make's `-j` parameter as you see fit
  - Sanity check `release-build/bin/rkt version`
- Add a signed tag: `git tag -s v0.7.0`. (We previously used tags for release notes, but now we store them in CHANGELOG.md, so a short tag with the release name is fine).
- Push the tag to GitHub: `git push --tags`

Now we switch to the GitHub web UI to conduct the release:
- https://github.com/coreos/rkt/releases/new
- Tag "v0.7.0", release title "v0.7.0"
- For now, check "This is a pre-release"
- Copy-paste the release notes you added earlier in [CHANGELOG.md](https://github.com/coreos/rkt/blob/master/CHANGELOG.md)
- You can also add a little more detail and polish to the release notes here if you wish, as it is more targeted towards users (vs the changelog being more for developers); use your best judgement and see previous releases on GH for examples.
- Attach the release. This is a simple tarball:

```
	export NAME="rkt-v0.7.0"
	mkdir $NAME
	cp release-build/bin/rkt release-build/bin/stage1-{coreos,kvm}.aci $NAME/
	tar czvf $NAME.tar.gz $NAME/
```

- Attach the release signature; your personal GPG is okay for now:

```
	gpg --detach-sign $NAME.tar.gz
```

- Publish the release!

Now it's announcement time: send an email to rkt-dev@googlegroups.com describing the release.
Generally this is higher level overview outlining some of the major features, not a copy-paste of the release notes.
Use your discretion and see [previous release emails](https://groups.google.com/forum/#!forum/rkt-dev) for examples.
Make sure to include a list of authors that contributed since the previous release - something like the following might be handy:

```
	git log ...v0.6.1 --pretty=format:"%an" | sort | uniq | tr '\n' ',' | sed -e 's#,#, #g' -e 's#, $##'
```
