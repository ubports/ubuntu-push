GOPATH := $(shell cd ../../..; pwd)
export GOPATH

PROJECT = launchpad.net/ubuntu-push

ifneq ($(CURDIR),$(GOPATH)/src/launchpad.net/ubuntu-push)
$(error unexpected curdir and/or layout)
endif

GODEPS = launchpad.net/gocheck

bootstrap:
	mkdir -p $(GOPATH)/bin
	mkdir -p $(GOPATH)/pkg
	go get -u launchpad.net/godeps
	go get -d -u $(GODEPS)
	$(GOPATH)/bin/godeps -u dependencies.tsv
	go install $(GODEPS)

check:
	go test -tags testing $(PROJECT)/...

check-race:
	go test -tags testing -race $(PROJECT)/...

coverage-summary:
	go test -tags testing -a -cover $(PROJECT)/...

coverage-html:
	mkdir -p coverhtml
	for pkg in $$(go list $(PROJECT)/...|grep -v acceptance ); do \
		relname="$${pkg#$(PROJECT)/}" ; \
		mkdir -p coverhtml/$$(dirname $${relname}) ; \
		go test -a -coverprofile=coverhtml/$${relname}.out $$pkg ; \
		if [ -f coverhtml/$${relname}.out ] ; then \
	           go tool cover -html=coverhtml/$${relname}.out -o coverhtml/$${relname}.html ; \
	           go tool cover -func=coverhtml/$${relname}.out -o coverhtml/$${relname}.txt ; \
		fi \
	done

format:
	go fmt $(PROJECT)/...

check-format:
	scripts/check_fmt $(PROJECT)

.PHONY: bootstrap check check-race format check-format coverage-summary \
	coverage-html
