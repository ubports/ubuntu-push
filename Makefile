GOPATH := $(shell cd ../../..; pwd)
export GOPATH

PROJECT = launchpad.net/ubuntu-push

ifneq ($(CURDIR),$(GOPATH)/src/launchpad.net/ubuntu-push)
$(error unexpected curdir and/or layout)
endif

GODEPS = launchpad.net/gocheck
GODEPS += launchpad.net/go-dbus/v1
GODEPS += launchpad.net/go-xdg/v0
GODEPS += code.google.com/p/gosqlite/sqlite3
GODEPS += code.google.com/p/go-uuid/uuid

TOTEST = $(shell env GOPATH=$(GOPATH) go list $(PROJECT)/...|grep -v acceptance|grep -v http13client )
TOBUILD = $(shell grep -lr '^package main')

all: fetchdeps bootstrap build-client build-server-dev

fetchdeps: .has-fetched-deps

.has-fetched-deps: PACKAGE_DEPS
	@$(MAKE) --no-print-directory refetchdeps
	@touch $@

refetchdeps:
	sudo apt-get install $$( grep -v '^#' PACKAGE_DEPS )

bootstrap: dependencies.tsv
	$(RM) -r $(GOPATH)/pkg
	mkdir -p $(GOPATH)/bin
	mkdir -p $(GOPATH)/pkg
	go get -u launchpad.net/godeps
	go get -d -u $(GODEPS)
	$(GOPATH)/bin/godeps -u dependencies.tsv
	go install $(GODEPS)

dependencies.tsv:
	$(GOPATH)/bin/godeps -t $(TOTEST) $(foreach i,$(TOBUILD),$(dir $(PROJECT)/$(i))) 2>/dev/null | cat > $@

check:
	go test $(TESTFLAGS) $(TOTEST)

check-race:
	go test $(TESTFLAGS) -race $(TOTEST)

acceptance:
	cd server/acceptance; ./acceptance.sh

build-client: ubuntu-push-client signing-helper/signing-helper

%.deps: %
	$(SH) scripts/deps.sh $<

%: %.go
	go build -o $@ $<

include $(TOBUILD:.go=.go.deps)

signing-helper/Makefile: signing-helper/CMakeLists.txt signing-helper/signing-helper.cpp signing-helper/signing.h
	cd signing-helper && (make clean || true) && cmake .

signing-helper/signing-helper: signing-helper/Makefile signing-helper/signing-helper.cpp signing-helper/signing.h
	cd signing-helper && make

build-server-dev: push-server-dev

run-server-dev: push-server-dev
	./$< sampleconfigs/dev.json

push-server-dev: server/dev/server
	mv $< $@

# very basic cleanup stuff; needs more work
clean:
	$(RM) -r coverhtml
	$(MAKE) -C signing-helper clean || true
	$(RM) push-server-dev
	$(RM) $(TOBUILD:.go=)

distclean:
	bzr clean-tree --verbose --ignored --force

coverage-summary:
	go test $(TESTFLAGS) -a -cover $(TOTEST)

coverage-html:
	mkdir -p coverhtml
	for pkg in $(TOTEST); do \
		relname="$${pkg#$(PROJECT)/}" ; \
		mkdir -p coverhtml/$$(dirname $${relname}) ; \
		go test $(TESTFLAGS) -a -coverprofile=coverhtml/$${relname}.out $$pkg ; \
		if [ -f coverhtml/$${relname}.out ] ; then \
	           go tool cover -html=coverhtml/$${relname}.out -o coverhtml/$${relname}.html ; \
	           go tool cover -func=coverhtml/$${relname}.out -o coverhtml/$${relname}.txt ; \
		fi \
	done

format:
	go fmt $(PROJECT)/...

check-format:
	scripts/check_fmt $(PROJECT)

protocol-diagrams: protocol/state-diag-client.svg protocol/state-diag-session.svg
%.svg: %.gv
	# requires graphviz installed
	dot -Tsvg $< > $@

.PHONY: bootstrap check check-race format check-format \
	acceptance build-client build-server-dev run-server-dev \
	coverage-summary coverage-html protocol-diagrams \
	fetchdeps refetchdeps clean distclean all

.INTERMEDIATE: server/dev/server
