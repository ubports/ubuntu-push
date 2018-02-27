# run acceptance tests, expects properly setup GOPATH and deps
# can set extra build params like -race with BUILD_FLAGS envvar
# can set server pkg name with SERVER_PKG
set -ex
go test $BUILD_FLAGS -i github.com/ubports/ubuntu-push/server/acceptance
go build $BUILD_FLAGS -o testserver github.com/ubports/ubuntu-push/server/dev
go test $BUILD_FLAGS -server ./testserver $*
