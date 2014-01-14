# run acceptance tests, expects properly setup GOPATH and deps
# can set extra build params like -race with BUILD_FLAGS envvar
set -ex
go test $BUILD_FLAGS -i
go build $BUILD_FLAGS -o testserver launchpad.net/ubuntu-push/server/dev
go test $BUILD_FLAGS -server ./testserver $*
