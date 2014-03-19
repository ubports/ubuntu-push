# help massage and extract from go development 1.3 net/http the client bits

grab:
	cp $(CHECKOUT)/src/pkg/net/http/*.go .
	mkdir -p httptest
	mkdir -p httputil
	mkdir -p testdata
	cp $(CHECKOUT)/src/pkg/net/http/httptest/*.go httptest
	cp $(CHECKOUT)/src/pkg/net//http/httputil/*.go httputil
	cp $(CHECKOUT)/src/pkg/net/http/testdata/* testdata
	hg -R $(CHECKOUT) summary > _using.txt

full-prepare:
	patch -R -p5 < _patches/sync_pool.Rpatch
	patch -p1 < _patches/no_keepalive.patch
	sed -i -e 's+"net/http"+"launchpad.net/ubuntu-push/http13client"+' *.go httptest/*.go httputil/*.go
	sed -i -e 's+"net/http/+"launchpad.net/ubuntu-push/http13client/+' *.go httptest/*.go httputil/*.go
	patch -p1 < _patches/no_serve_test_unsupported_bench.patch


clean:
	rm -rf *.go httptest httputil testdata

.PHONY: grab full-prepare clean