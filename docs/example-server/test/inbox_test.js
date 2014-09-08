var assert = require('assert')

var MongoClient = require('mongodb').MongoClient

function unexpected(msg) {
    assert.ok(false, "unexpected: "+msg)
}

var Inbox = require('../lib/inbox')

suite('Inbox', function(){
    setup(function(done) {
        var self = this
        MongoClient.connect("mongodb://localhost:27017/pushAppTestDb", function(err, database) {
            if(err) throw err
            self.db = database
            // cleanup
            self.db.collection('inbox').drop(function(err) {
                if(err && err.errmsg != 'ns not found') throw err
                done()
            })
        })
    })

    test('push-1', function(done) {
        var inbox = new Inbox(this.db)
        inbox.pushMessage('foo', {m: 42}, function(msg) {
            assert.ok(msg._timestamp)
            assert.equal(msg._serial, 1)
            done()
        }, function(err) {
            unexpected(err)
        })
    })

    test('push-2', function(done) {
        var inbox = new Inbox(this.db)
        inbox.pushMessage('foo', {m: 42}, function(msg) {
            assert.equal(msg._serial, 1)
            inbox.pushMessage('foo', {m: 45}, function(msg) {
                assert.equal(msg._serial, 2)
                done()
            }, function(err) {
                unexpected(err)
            })
        }, function(err) {
            unexpected(err)
        })
    })

    test('push-3-drain', function(done) {
        var inbox = new Inbox(this.db)
        inbox.pushMessage('foo', {m: 42, _timestamp: 2000}, function(msg) {
            inbox.pushMessage('foo', {m: 45, _timestamp: 3000}, function(msg) {
                inbox.pushMessage('foo', {m: 47, _timestamp: 4000}, function(msg) {
                    inbox.drain('foo', 3000, function(msgs) {
                        assert.deepEqual(msgs, [
                            {m: 45, _timestamp: 3000, serial: 2},
                            {m: 47, _timestamp: 4000, serial: 3}
                        ])
                        inbox.drain('foo', 0, function(msgs) {
                            assert.equal(msgs.length, 2)
                            done()
                        }, done)
                    }, done)
                }, done)
            }, done)
        }, done)
    })

    test('drain-nop', function(done) {
        var inbox = new Inbox(this.db)
        inbox.drain('foo', 3000, function(msgs) {
            assert.deepEqual(msgs, [])
            done()
        }, done)
    })

})