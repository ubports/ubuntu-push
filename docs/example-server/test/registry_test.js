var assert = require('assert')

var MongoClient = require('mongodb').MongoClient

function unexpected(msg) {
    assert.ok(false, "unexpected: "+msg)
}

var Registry = require('../lib/registry')

suite('Registry', function(){
    setup(function(done) {
        var self = this
        MongoClient.connect("mongodb://localhost:27017/pushAppTestDb", function(err, database) {
            if(err) throw err
            self.db = database
            // cleanup
            self.db.collection('registry').drop(function(err) {
                if(err && err.errmsg != 'ns not found') throw err
                done()
            })
        })
    })

    test('insert-and-find', function(done) {
        var reg = new Registry(this.db)
        reg.insertToken("N", "T", function() {
            reg.findToken("N", function(token) {
                assert.equal(token, "T")
                done()
            }, function() {
                unexpected("not-found")
            }, function(err) {
                unexpected(err)
            })
        }, function() {
            unexpected("dup")
        }, function(err) {
            unexpected(err)
        })
    })

    test('find-not-found', function(done) {
        var reg = new Registry(this.db)
        reg.findToken("N", function() {
            unexpected("found")
        }, function() {
            done()
        }, function(err) {
            unexpected(err)
        })
    })

    test('insert-identical-dup', function(done) {
        var reg = new Registry(this.db)
        reg.insertToken("N", "T", function() {
            reg.insertToken("N", "T", function() {
                done()
            }, function() {
                unexpected("dup")
            }, function(err) {
                unexpected(err)
            })
        }, function() {
            unexpected("dup")
        }, function(err) {
            unexpected(err)
        })
    })

    test('insert-dup', function(done) {
        var reg = new Registry(this.db)
        reg.insertToken("N", "T1", function() {
            reg.insertToken("N", "T2", function() {
                unexpected("success")
            }, function() {
                done()
            }, function(err) {
                unexpected(err)
            })
        }, function() {
            unexpected("dup")
        }, function(err) {
            unexpected(err)
        })
    })

    test('insert-temp-dup', function(done) {
        var reg = new Registry(this.db)
        var findToken = reg.findToken
          , insertToken = reg.insertToken
        var notFoundOnce = 0
        var insertInvocations = 0
        reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            if (notFoundOnce == 0) {
                notFoundOnce++
                notFoundCb()
                return
            }
            findToken.call(reg, nick, foundCb, notFoundCb, errCb)
        }
        reg.insertToken = function(nick, token, doneCb, dupCb, errCb) {
            insertInvocations++
            insertToken.call(reg, nick, token, doneCb, dupCb, errCb)
        }
        reg.insertToken("N", "T1", function() {
            reg.insertToken("N", "T2", function() {
                unexpected("success")
            }, function() {
                assert.equal(insertInvocations, 3)
                done()
            }, function(err) {
                unexpected(err)
            })
        }, function() {
            unexpected("dup")
        }, function(err) {
            unexpected(err)
        })
    })

    test('remove', function(done) {
        var reg = new Registry(this.db)
        reg.insertToken("N", "T", function() {
            reg.removeToken("N", "T", function() {
                reg.findToken("N", function(token) {
                    unexpected("found")
                }, function() {
                    done()
                }, function(err) {
                    unexpected(err)
                })
            }, function(err) {
                unexpected(err)
            })
        },  function() {
            unexpected("dup")
        }, function(err) {
            unexpected(err)
        })
    })

    test('remove-exact', function(done) {
        var reg = new Registry(this.db)
        reg.insertToken("N", "T1", function() {
            reg.removeToken("N", "T2", function() {
                reg.findToken("N", function(token) {
                    assert.equal(token, "T1")
                    done()
                }, function() {
                    unexpected("no-found")
                }, function(err) {
                    unexpected(err)
                })
            }, function(err) {
                unexpected(err)
            })
        },  function() {
            unexpected("dup")
        }, function(err) {
            unexpected(err)
        })
    })

    test('remove-nop', function(done) {
        var reg = new Registry(this.db)
        reg.removeToken("N1", "T1", function() {
            reg.findToken("N1", function(token) {
                unexpected("found")
            }, function() {
                done()
            }, function(err) {
                unexpected(err)
            })
        }, function(err) {
            unexpected(err)
        })
    })
})
