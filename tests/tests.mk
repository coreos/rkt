$(call setup-stamp-file,TST_SHORT_TESTS_STAMP,/short-tests)

TST_DIRS_WITH_GOFILES := $(call go-find-directories,$(GO_TEST_PACKAGES),GoFiles)
TST_DIRS_WITH_TESTGOFILES := $(call go-find-directories,$(GO_TEST_PACKAGES),TestGoFiles,tests)

# gofmt takes list of directories
TST_GOFMT_DIRS := $(foreach d,$(TST_DIRS_WITH_GOFILES),./$d)
# go vet and go test take a list of packages
TST_GO_VET_PACKAGES := $(foreach d,$(TST_DIRS_WITH_GOFILES),$(REPO_PATH)/$d)
TST_GO_TEST_PACKAGES := $(foreach d,$(TST_DIRS_WITH_TESTGOFILES),$(REPO_PATH)/$d)

# Workaround for https://github.com/golang/go/issues/6820
# Disable go vet on Semaphore for store/store_test.go
# TODO: remove this when Semaphore updates their go installation
ifeq ($(SEMAPHORE),true)
TST_GO_VET_PACKAGES := $(subst github.com/coreos/rkt/store,,$(TST_GO_VET_PACKAGES))
endif

$(TST_SHORT_TESTS_STAMP): TST_GOFMT_DIRS := $(TST_GOFMT_DIRS)
$(TST_SHORT_TESTS_STAMP): TST_GO_VET_PACKAGES := $(TST_GO_VET_PACKAGES)
$(TST_SHORT_TESTS_STAMP): TST_GO_TEST_PACKAGES := $(TST_GO_TEST_PACKAGES)
$(TST_SHORT_TESTS_STAMP):
	set -e; \
	res=$$($(GOFMT) -l $(TST_GOFMT_DIRS)); \
	if [ -n "${res}" ]; then echo -e "gofmt checking failed:\n${res}"; exit 1; fi; \
	res=$$($(GO_ENV) "$(GO)" vet $(TST_GO_VET_PACKAGES)); \
	if [ -n "${res}" ]; then echo -e "govet checking failed:\n${res}"; exit 1; fi; \
	res=$$( \
		for file in $$(find . -type f -iname '*.go' ! -path './Godeps/*'); do \
			head -n1 "$${file}" | grep -Eq "(Copyright|generated)" || echo -e "  $${file}"; \
		done; \
	); \
	if [ -n "${res}" ]; then echo -e "license header checking failed:\n${res}"; exit 1; fi; \
	$(GO_ENV) "$(GO)" test -timeout 60s -cover $(TST_GO_TEST_PACKAGES) --race

TOPLEVEL_CHECK_STAMPS += $(TST_SHORT_TESTS_STAMP)

ifeq ($(RKT_RUN_FUNCTIONAL_TESTS),yes)

$(call inc-one,functional.mk)

endif

TST_SHORT_TESTS_STAMP :=
TST_DIRS_WITH_GOFILES :=
TST_DIRS_WITH_TESTGOFILES :=
TST_GOFMT_DIRS :=
TST_GO_VET_PACKAGES :=
TST_GO_TEST_PACKAGES :=
