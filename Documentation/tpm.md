# rkt and TPMs

rkt supports measuring container state and configuration into the TPM event
log. Enable this functionality by building rkt with the --enable-tpm=yes
argument to configure. rkt accesses the TPM via the tpmd executable
available from https://github.com/coreos/go-tspi and assumes that tpmd is
listening on port 12041. Events will be logged to PCR 15 with event type
0x1000, and contain the following data:

1) The hash of the container root
2) The contents of the container manifest data
3) The arguments passed to stage 1

This provides a cryptographically verifiable audit log of which containers a
node has booted and the full configuration passed to that container.