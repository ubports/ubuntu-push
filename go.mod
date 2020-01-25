module github.com/ubports/ubuntu-push

go 1.13

require (
	github.com/mattn/go-sqlite3 v1.1.1-0.20160927022846-4b0af852c171
	github.com/pborman/uuid v0.0.0-20160824210600-b984ec7fa9ff
	launchpad.net/go-dbus v1.0.0-20140208094800-gubd5md7cro3mtxa
	launchpad.net/go-xdg v0.0.0-20140208094800-000000000010
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087
)

replace launchpad.net/go-dbus => github.com/z3ntu/go-dbus v0.0.0-20170220120108-c022b8b2e127
