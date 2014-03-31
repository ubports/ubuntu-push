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

TOTEST = $(shell env GOPATH=$(GOPATH) go list $(PROJECT)/...|grep -v acceptance{|grep -v http13client )

bootstrap:
	mkdir -p $(GOPATH)/bin
	mkdir -p $(GOPATH)/pkg
	go get -u launchpad.net/godeps
	go get -d -u $(GODEPS)
	$(GOPATH)/bin/godeps -u dependencies.tsv
	go install $(GODEPS)

check:
	go test $(TESTFLAGS) $(TOTEST)

check-race:
	go test $(TESTFLAGS) -race $(TOTEST)

acceptance:
	cd server/acceptance; ./acceptance.sh

build-client:
	go build ubuntu-push-client.go

build-server-dev:
	go build -o push-server-dev launchpad.net/ubuntu-push/server/dev

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
	acceptance build-client bluild-server-dev \
	coverage-summary coverage-html protocol-diagrams
