PACKAGE  = stash.kopano.io/kwm/kwmserver
PACKAGE_NAME = kopano-$(shell basename $(PACKAGE))

# Tools

GO      ?= go
GOFMT   ?= gofmt
GOLINT  ?= golangci-lint
DLV     ?= dlv

GO2XUNIT ?= go2xunit
GOCOV    ?= gocov
GOCOVXML ?= gocov-xml
GOCOVMERGE ?= gocovmerge

CHGLOG ?= git-chglog

# Cgo

CGO_ENABLED ?= 0

# Go modules

GO111MODULE ?= on

# Variables

export CGO_ENABLED GO111MODULE
unexport GOPATH

ARGS    ?=
PWD     := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2>/dev/null | sed 's/^v//' || \
			cat $(CURDIR)/.version 2> /dev/null || echo 0.0.0-unreleased)
PKGS     = $(or $(PKG),$(shell $(GO) list -mod=readonly ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS = $(shell $(GO) list -mod=readonly -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS) 2>/dev/null)
CMDS     = $(or $(CMD),$(addprefix cmd/,$(notdir $(shell find "$(PWD)/cmd/" -type d))))
TIMEOUT ?= 240

# Build

.PHONY: all
all: fmt vendor | $(CMDS)

.PHONY: $(CMDS)
$(CMDS): vendor ; $(info building $@ ...) @
	$(GO) build \
		-mod vendor \
		-trimpath \
		-tags release \
		-buildmode=exe \
		-ldflags '-s -w -buildid=reproducible/$(VERSION) -X $(PACKAGE)/version.Version=$(VERSION) -X $(PACKAGE)/version.BuildDate=$(DATE) -extldflags -static' \
		-o bin/$(notdir $@) ./$@

# Helpers

.PHONY: lint
lint: vendor ; $(info running $(GOLINT) ...)	@
	$(GOLINT) run

.PHONY: lint-checkstyle
lint-checkstyle: vendor ; $(info running $(GOLINT) checkstyle ...)     @
	@mkdir -p test
	$(GOLINT) run --out-format checkstyle --issues-exit-code 0 > test/tests.lint.xml

.PHONY: fmt
fmt: ; $(info running gofmt ...)	@
	@ret=0 && for d in $$($(GO) list -mod=readonly -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GOFMT) -l -w $$d/*.go || ret=$$? ; \
	done ; exit $$ret

.PHONY: check
check: ; $(info checking dependencies ...) @
	@$(GO) mod verify && echo OK

# Tests

TEST_TARGETS := test-default test-bench test-short test-race test-verbose
.PHONY: $(TEST_TARGETS)
test-bench:   ARGS=-run=_Bench* -test.benchmem -bench=.
test-short:   ARGS=-short
test-race:    ARGS=-race
test-race:    CGO_ENABLED=1
test-verbose: ARGS=-v
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test

.PHONY: test
test: ; $(info running $(NAME:%=% )tests ...)	@
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

TEST_XML_TARGETS := test-xml-default test-xml-short test-xml-race
.PHONY: $(TEST_XML_TARGETS)
test-xml-short: ARGS=-short
test-xml-race:  ARGS=-race
test-xml-race:  CGO_ENABLED=1
$(TEST_XML_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_XML_TARGETS): test-xml

.PHONY: test-xml
test-xml: ; $(info running $(NAME:%=% )tests ...)	@
	@mkdir -p test
	2>&1 CGO_ENABLED=1 $(GO) test -timeout $(TIMEOUT)s $(ARGS) -v $(TESTPKGS) | tee test/tests.output
	test -s test/tests.output && $(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML = $(COVERAGE_DIR)/coverage.html
.PHONY: test-coverage
test-coverage: COVERAGE_DIR := $(CURDIR)/test/coverage.$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
test-coverage: ; $(info running coverage tests ...)
	@mkdir -p $(COVERAGE_DIR)/coverage
	@rm -f test/tests.output
	@for pkg in $(TESTPKGS); do \
		CGO_ENABLED=$(CGO_ENABLED) $(GO) test -timeout $(TIMEOUT)s -v \
			-coverpkg=$$($(GO) list -mod=readonly -f '{{ join .Deps "\n" }}' $$pkg | \
					grep '^$(PACKAGE)/' | grep -v '^$(PACKAGE)/vendor/' | \
					tr '\n' ',')$$pkg \
			-covermode=atomic \
			-coverprofile="$(COVERAGE_DIR)/coverage/`echo $$pkg | tr "/" "-"`.cover" $$pkg | tee -a test/tests.output ;\
	done
	@$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml
	@$(GOCOVMERGE) $(COVERAGE_DIR)/coverage/*.cover > $(COVERAGE_PROFILE)
	@$(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	@$(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

# Debug

.PHONY: dlv
dlv: ; $(info attaching Delve debugger ...)
	$(DLV) attach --api-version=$(DLV_APIVERSION) $(DLV_ARGS) $(DLV_ATTACH_PID) $(DLV_EXECUTABLE)

# Mod

go.sum: go.mod ; $(info updating dependencies ...)
	@$(GO) mod tidy -v
	@touch $@

.PHONY: vendor
vendor: go.sum ; $(info retrieving dependencies ...)
	@$(GO) mod vendor -v
	@touch $@

# Dist

.PHONY: licenses
licenses: ; $(info building licenses files ...)
	$(CURDIR)/scripts/go-license-ranger.py > $(CURDIR)/3rdparty-LICENSES.md

3rdparty-LICENSES.md: licenses

.PHONY: dist
dist: 3rdparty-LICENSES.md ; $(info building dist tarball ...)
	@rm -rf "dist/${PACKAGE_NAME}-${VERSION}"
	@mkdir -p "dist/${PACKAGE_NAME}-${VERSION}"
	@mkdir -p "dist/${PACKAGE_NAME}-${VERSION}/scripts"
	@cd dist && \
	cp -avf ../LICENSE.txt "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../README.md "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../3rdparty-LICENSES.md "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../registration.yaml.in "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../bin/* "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../scripts/kopano-kwmserverd.binscript "${PACKAGE_NAME}-${VERSION}/scripts" && \
	cp -avf ../scripts/kopano-kwmserverd.service "${PACKAGE_NAME}-${VERSION}/scripts" && \
	cp -avf ../scripts/kwmserverd.cfg "${PACKAGE_NAME}-${VERSION}/scripts" && \
	mkdir -p "${PACKAGE_NAME}-${VERSION}/docs" && \
	cp -avr ../docs/api-v1 "${PACKAGE_NAME}-${VERSION}/docs" && \
	mkdir -p "${PACKAGE_NAME}-${VERSION}/www" && \
	cp -avr ../www/examples "${PACKAGE_NAME}-${VERSION}/www" && \
	tar --owner=0 --group=0 -czvf ${PACKAGE_NAME}-${VERSION}.tar.gz "${PACKAGE_NAME}-${VERSION}" && \
	cd ..

.PHONE: changelog
changelog: ; $(info updating changelog ...)
	$(CHGLOG) --output CHANGELOG.md $(ARGS)

# Rest

.PHONY: clean
clean: ; $(info cleaning ...)	@
	@rm -rf bin
	@rm -rf test/test.*

.PHONY: version
version:
	@echo $(VERSION)
