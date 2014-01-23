# run acceptance tests, expects properly setup GOPATH and deps
# can set extra build params like -race with BUILD_FLAGS envvar
# can set server pkg name with SERVER_PKG
set -ex
SERVER_PKG=${SERVER_PKG:-launchpad.net/ubuntu-push/server/dev}
go test $BUILD_FLAGS -i launchpad.net/ubuntu-push/server/acceptance
go build $BUILD_FLAGS -o testserver ${SERVER_PKG}
cd ${GOPATH}/src/launchpad.net/ubuntu-push/server/acceptance
go test $BUILD_FLAGS -server ${PWD}/testserver $*
