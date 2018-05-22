# Makefile used to create packages for diva. It doesn't assume that the code is
# inside a GOPATH, and always copy the files into a new workspace to get the
# work done. Go tools doesn't reliably work with symbolic links.
#
# For historical purposes, it also works in a development environment when the
# repository is already inside a GOPATH.
.NOTPARALLEL:

GO_PACKAGE_PREFIX := github.com/clearlinux/diva

.PHONY: gopath

# Strictly speaking we should check if it the directory is inside an
# actual GOPATH, but the directory structure matching is likely enough.
ifeq (,$(findstring ${GO_PACKAGE_PREFIX},${CURDIR}))
LOCAL_GOPATH := ${CURDIR}/.gopath
export GOPATH := ${LOCAL_GOPATH}
gopath:
	@rm -rf ${LOCAL_GOPATH}/src
	@mkdir -p ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
	@cp -af * ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
	@echo "Prepared a local GOPATH=${GOPATH}"
else
LOCAL_GOPATH :=
GOPATH ?= ${HOME}/go
gopath:
	@echo "Code already in existing GOPATH=${GOPATH}"
endif

.PHONY: build install clean check

.DEFAULT_GOAL := build


build: gopath
	go install ${GO_PACKAGE_PREFIX}

install: gopath
	test -d $(DESTDIR)/usr/bin || install -D -d -m 00755 $(DESTDIR)/usr/bin;
	install -m 00755 $(GOPATH)/bin/diva $(DESTDIR)/usr/bin/.

check: gopath
	go test -cover ${GO_PACKAGE_PREFIX}/...

# TODO: since Go 1.10 we have support for passing multiple packages
# to coverprofile. Update this to work on all packages.
.PHONY: checkcoverage
checkcoverage: gopath
ifeq (,${PKG})
	$(error PKG is not set, try make PKG=internal/config checkcoverage)
else
	go test -cover ${GO_PACKAGE_PREFIX}/${PKG} -coverprofile=coverage.out
	go tool cover -html=coverage.out
endif

.PHONY: lint
lint: gopath
	@gometalinter.v2 --deadline=10m --tests --vendor --disable-all \
	--enable=misspell \
	--enable=vet \
	--enable=ineffassign \
	--enable=gofmt \
	$${CYCLO_MAX:+--enable=gocyclo --cyclo-over=$${CYCLO_MAX}} \
	--enable=golint \
	--enable=deadcode \
	--enable=varcheck \
	--enable=structcheck \
	--enable=unused \
	--enable=vetshadow \
	--enable=errcheck \
	./...

clean:
ifeq (,${LOCAL_GOPATH})
	go clean -i -x
else
	rm -rf ${LOCAL_GOPATH}
endif
	rm -f diva-*.tar.gz

release:
	@if [ ! -d .git ]; then \
		echo "Release needs to be used from a git repository"; \
		exit 1; \
	fi
	@VERSION=$$(grep -e 'const version' main.go | cut -d '"' -f 2) ; \
	if [ -z "$$VERSION" ]; then \
		echo "Couldn't extract version number from the source code"; \
		exit 1; \
	fi; \
	git archive --format=tar.gz --verbose -o diva-$$VERSION.tar.gz HEAD --prefix=diva-$$VERSION/
