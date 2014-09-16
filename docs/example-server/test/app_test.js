var assert = require('assert')
  ,  http = require('http')
  ,  request = require('supertest')

var app = require('../app')

var cfg = {
    'app_id': 'app1',
    'push_url': 'http://push',
    'expire_mins': 10,
    'retry_secs': 0.05,
    'retry_batch': 1,
    'happy_retry_secs': 0.02,
}

function cloneCfg() {
    return JSON.parse(JSON.stringify(cfg))
}

var PLAY_NOTIFY_FORM = '/play-notify-form'

suite('app', function() {
    setup(function() {
        this.db = {}
        this.app = app.wire(this.db, cloneCfg())
        this.reg = this.app.get('_reg')
        this.inbox = this.app.get('_inbox')
        this.notifier = this.app.get('_notifier')
    })

    test('wire', function() {
        assert.ok(this.reg)
        assert.ok(this.notifier)
        assert.equal(this.notifier.baseURL, 'http://push')
        var emitted
        this.app.on('pushError', function(err, resp, body) {
            emitted = [err, resp, body]
        })
        this.notifier.pushError('err', 'resp', 'body')
        assert.deepEqual(emitted, ["err", "resp", "body"])
    })

    test('wire-unknownToken', function() {
        var got
        this.reg.removeToken = function(nick, token, doneCb, errCb) {
            got = [nick, token]
            doneCb()
        }
        this.notifier.emit('unknownToken', "N", "T")
        assert.deepEqual(got, ["N", "T"])
    })

    test('wire-unknownToken-mongoError', function() {
        var emitted
        this.app.on('mongoError', function(err) {
            emitted = err
        })
        this.reg.removeToken = function(nick, token, doneCb, errCb) {
            errCb({})
        }
        this.notifier.emit('unknownToken', "N", "T")
        assert.ok(emitted)
    })

    test('_check', function(done) {
        var pingCmd
        this.db.command = function(cmd, cb) {
            pingCmd = cmd
            cb(null)
        }
        request(this.app)
            .get('/_check')
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({ok: true})
            .expect(200, function(err) {
                assert.deepEqual(pingCmd, {ping: 1})
                done(err)
            })
    })

    test('_check-unavailable', function(done) {
        var pingCmd
        this.db.command = function(cmd, cb) {
            pingCmd = cmd
            cb({})
        }
        request(this.app)
            .get('/_check')
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: "unavailable"})
            .expect(503, function(err) {
                done(err)
            })
    })

    test('any-broken', function(done) {
        request(this.app)
            .post('/register')
            .set('Content-Type', 'application/json')
            .send("")
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'invalid', message: 'invalid json, empty body'})
            .expect(400, done)
    })

    test('register', function(done) {
        var got
        this.reg.insertToken = function(nick, token, doneCb, dupCb, errCb) {
            got = [nick, token]
            doneCb()
        }
        request(this.app)
            .post('/register')
            .set('Content-Type', 'application/json')
            .send({nick: "N", token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({ok: true})
            .expect(200, function(err) {
                assert.deepEqual(got, ["N", "T"])
                done(err)
            })
    })

    test('register-invalid', function(done) {
        request(this.app)
            .post('/register')
            .set('Content-Type', 'application/json')
            .send({token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'invalid'})
            .expect(400, done)
    })

    test('register-dup', function(done) {
        this.reg.insertToken = function(nick, token, doneCb, dupCb, errCb) {
            dupCb()
        }
        request(this.app)
            .post('/register')
            .set('Content-Type', 'application/json')
            .send({nick: "N", token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'dup'})
            .expect(400, done)
    })

    test('register-unavailable', function(done) {
        this.reg.insertToken = function(nick, token, doneCb, dupCb, errCb) {
            errCb({})
        }
        request(this.app)
            .post('/register')
            .set('Content-Type', 'application/json')
            .send({nick: "N", token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'unavailable'})
            .expect(503, done)
    })

    test('message', function(done) {
        var lookup = []
        var pushed
        var notify
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            lookup.push(nick)
            if (nick == "N") {
                foundCb("T")
            } else {
                foundCb("T2")
            }
        }
        this.inbox.pushMessage = function(token, msg, doneCb, errCb) {
            pushed = [token]
            msg._serial = 10
            doneCb(msg)
        }
        this.notifier.notify = function(nick, token, data) {
            notify = [nick, token, data]
        }
        request(this.app)
            .post('/message')
            .set('Content-Type', 'application/json')
            .send({nick: "N2", data: {"m": 1}, from_nick: "N", from_token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({ok: true})
            .expect(200, function(err) {
                assert.deepEqual(lookup, ["N", "N2"])
                assert.deepEqual(pushed, ["T2"])
                assert.deepEqual(notify, ["N2", "T2", {m: 1,_from: "N",_serial: 10}])
                done(err)
            })
    })

    test('message-unauthorized', function(done) {
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            if (nick == "N") {
                foundCb("T")
            } else {
                notFoundCb()
            }
        }
        request(this.app)
            .post('/message')
            .set('Content-Type', 'application/json')
            .send({nick: "N2", data: {"m": 1}, from_nick: "N", from_token: "Z"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: "unauthorized"})
            .expect(401, done)
    })

    test('message-unknown-nick', function(done) {
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            if (nick == "N") {
                foundCb("T")
            } else {
                notFoundCb()
            }
        }
        request(this.app)
            .post('/message')
            .set('Content-Type', 'application/json')
            .send({nick: "N2", data: {"m": 1}, from_nick: "N", from_token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: "unknown-nick"})
            .expect(400, done)
    })

    test('message-invalid', function(done) {
        request(this.app)
            .post('/message')
            .set('Content-Type', 'application/json')
            .send({nick: "N"}) // missing data
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'invalid'})
            .expect(400, done)
    })

    test('message-check-token-unavailable', function(done) {
        var emitted
        this.app.on('mongoError', function(err) {
            emitted = err
        })
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            if (nick == "N") {
                errCb({})
            } else {
                foundCb("T2")
            }
        }
        request(this.app)
            .post('/message')
            .set('Content-Type', 'application/json')
            .send({nick: "N2", data: {"m": 1}, from_nick: "N", from_token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'unavailable'})
            .expect(503, function(err) {
                assert.ok(emitted)
                done(err)
            })
    })

    test('message-inbox-unavailable', function(done) {
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            if (nick == "N") {
                foundCb("T")
            } else {
                foundCb("T2")
            }
        }
        this.inbox.pushMessage = function(token, msg, doneCb, errCb) {
            errCb({})
        }
        request(this.app)
            .post('/message')
            .set('Content-Type', 'application/json')
            .send({nick: "N2", data: {"m": 1}, from_nick: "N", from_token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'unavailable'})
            .expect(503, done)
    })

    test('message-notify-unavailable', function(done) {
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            if (nick == "N") {
                foundCb("T")
            } else {
                errCb({})
            }
        }
        request(this.app)
            .post('/message')
            .set('Content-Type', 'application/json')
            .send({nick: "N2", data: {"m": 1}, from_nick: "N", from_token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'unavailable'})
            .expect(503, function(err) {
                done(err)
            })
    })

    test('index', function(done) {
        request(this.app)
            .get('/')
            .expect(new RegExp('<title>pushAppServer'))
            .expect('Content-Type', 'text/html; charset=UTF-8')
            .expect(200, done)
    })

    test('play-notify-form-absent', function(done) {
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .type('form')
            .send({nick: "N", data: '{"m": 1}'})
            .expect(404, done)
    })

    test('drain', function(done) {
        var lookup
        var got
        var msgs = [{'m': 42, _timestamp: 4000, _serial: 20}]
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            lookup = [nick]
            foundCb("T")
        }
        this.inbox.drain = function(token, timestamp, doneCb, errCb) {
            got = [token, timestamp]
            doneCb(msgs)
        }
        request(this.app)
            .post('/drain')
            .set('Content-Type', 'application/json')
             .send({nick: "N", token: "T", timestamp: 4000})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({ok: true, msgs: msgs})
            .expect(200, function(err) {
                assert.deepEqual(lookup, ["N"])
                assert.deepEqual(got, ["T", 4000])
                done(err)
            })
    })

    test('drain-unavailable', function(done) {
        var msgs = [{'m': 42, _timestamp: 4000, _serial: 20}]
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            foundCb("T")
        }
        this.inbox.drain = function(token, timestamp, doneCb, errCb) {
            errCb()
        }
        request(this.app)
            .post('/drain')
            .set('Content-Type', 'application/json')
             .send({nick: "N", token: "T", timestamp: 4000})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'unavailable'})
            .expect(503, function(err) {
                done(err)
            })
    })

    test('drain-invalid', function(done) {
        request(this.app)
            .post('/drain')
            .set('Content-Type', 'application/json')
            .send({nick: "N"}) // missing data
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'invalid'})
            .expect(400, done)
    })

    test('drain-invalid-timestamp', function(done) {
        request(this.app)
            .post('/drain')
            .set('Content-Type', 'application/json')
            .send({nick: "N", token: "T", timestamp: "foo"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({error: 'invalid'})
            .expect(400, done)
    })

})

suite('app-with-play-notify', function() {
    setup(function() {
        this.db = {}
        var cfg = cloneCfg()
        cfg.play_notify_form = true
        this.app = app.wire(this.db, cfg)
        this.reg = this.app.get('_reg')
        this.notifier = this.app.get('_notifier')
    })

    test('root-form', function(done) {
        request(this.app)
            .get('/')
            .expect(new RegExp('<form.*action="' + PLAY_NOTIFY_FORM + '"(.|\n)*<input  class="form-control" placeholder="Message destination" id="nick" name="nick" type="text"'))
            .expect('Content-Type', 'text/html; charset=UTF-8')
            .expect(200, done)
    })

    test('play-notify-form', function(done) {
        var notify
          , start = Date.now()
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            foundCb("T")
        }
        this.notifier.notify = function(nick, token, data) {
            notify = [nick, token, data]
        }
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .type('form')
            .send({nick: "N", message: 'foo'})
            .expect('Content-Type', 'text/plain; charset=utf-8')
            .expect('Moved Temporarily. Redirecting to /')
            .expect(302, function(err) {
                assert.deepEqual(notify.slice(0, 2), ["N", "T"])
                var data = notify[2]
                assert.equal(typeof(data._ephemeral), "number")
                assert.ok(data._ephemeral >= start)
                delete data._ephemeral
                assert.deepEqual(data, {"message":{"from":"website","message":"foo","to":"n"},"notification":{}})
                done(err)
            })
    })

    test('play-notify-form-unknown-nick', function(done) {
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            notFoundCb()
        }
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .set('Content-Type', 'application/json')
            .send({nick: "N", message: 'foo'})
            .expect('Content-Type', 'text/plain; charset=utf-8')
            .expect("Moved Temporarily. Redirecting to /?error=unknown%20nick")
            .expect(302, done)
    })

    test('play-notify-form-unavailable', function(done) {
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            errCb({})
        }
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .type('form')
            .send({nick: "N", message: 'foo'})
            .expect('{"error":"unavailable"}')
            .expect(503, function(err) {
                done(err)
            })
    })

    test('play-notify-form-invalid', function(done) {
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            notFoundCb()
        }
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .set('Content-Type', 'application/json')
            .send({nick: "", message: 'foo'})
            .expect('Content-Type', 'text/plain; charset=utf-8')
            .expect('Moved Temporarily. Redirecting to /?error=invalid%20or%20empty%20fields%20in%20form')
            .expect(302, done)
    })

    test('play-notify-form-broken', function(done) {
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .type('form')
            .send("=")
            .expect('Moved Temporarily. Redirecting to /?error=invalid%20or%20empty%20fields%20in%20form')
            .expect(302, done)
    })

    test('play-notify-form-nick-is-lowercased', function(done) {
        var notify
          , start = Date.now()
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            foundCb("T")
        }
        this.notifier.notify = function(nick, token, data) {
            notify = [nick, token, data]
        }
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .type('form')
            .send({nick: "N", message: 'foo'})
            .expect('Content-Type', 'text/plain; charset=utf-8')
            .expect('Moved Temporarily. Redirecting to /')
            .expect(302, function(err) {
                assert.equal(notify[2]["message"]["to"], "n")
                done(err)
                }
            )
    })

    test('play-notify-disabled-notifications', function(done) {
        var notify
          , start = Date.now()
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            foundCb("T")
        }
        this.notifier.notify = function(nick, token, data) {
            notify = [nick, token, data]
        }
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .type('form')
            .send({nick: "N", message: 'foo', persist: 'on'})
            .expect('Content-Type', 'text/plain; charset=utf-8')
            .expect('Moved Temporarily. Redirecting to /')
            .expect(302, function(err) {
                assert.deepEqual(notify[2]["notification"], {})
                done(err)
                }
            )
    })
    test('play-notify-enabled-notifications', function(done) {
        var notify
          , start = Date.now()
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            foundCb("T")
        }
        this.notifier.notify = function(nick, token, data) {
            notify = [nick, token, data]
        }
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .type('form')
            .send({nick: "N", message: 'foo', enable: 'on', persist: 'on'})
            .expect('Content-Type', 'text/plain; charset=utf-8')
            .expect('Moved Temporarily. Redirecting to /')
            .expect(302, function(err) {
                assert.deepEqual(notify[2]["notification"], {
                    card: {
                        summary: 'The website says:',
                        body: 'foo',
                        actions: [ 'appid://com.ubuntu.developer.ralsina.hello/hello/current-user-version' ],
                        persist: true
                    }})
                done(err)
                }
            )
    })

    test('play-notify-enabled-all-notifications', function(done) {
        var notify
          , start = Date.now()
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            foundCb("T")
        }
        this.notifier.notify = function(nick, token, data) {
            notify = [nick, token, data]
        }
        request(this.app)
            .post(PLAY_NOTIFY_FORM)
            .type('form')
            .send({
                nick: "N",
                message: 'foo',
                enable: 'on',
                popup: 'on',
                persist: 'on',
                sound: 'on',
                vibrate: 'on',
                counter: 42
            })
            .expect('Content-Type', 'text/plain; charset=utf-8')
            .expect('Moved Temporarily. Redirecting to /')
            .expect(302, function(err) {
                assert.deepEqual(notify[2]["notification"], {
                    card: {
                        summary: 'The website says:',
                        body: 'foo',
                        actions: [ 'appid://com.ubuntu.developer.ralsina.hello/hello/current-user-version' ],
                        popup: true,
                        persist: true
                    },
                    sound: true,
                    vibrate: { duration: 200 },
                    'emblem-counter': { count: 42, visible: true}
                })
                done(err)
                }
            )
    })

})

suite('app-with-no-inbox', function() {
    setup(function() {
        this.db = {}
        var cfg = cloneCfg()
        cfg.no_inbox = true
        this.app = app.wire(this.db, cfg)
        this.reg = this.app.get('_reg')
        this.inbox = this.app.get('_inbox')
        this.notifier = this.app.get('_notifier')
    })

    test('no-inbox', function() {
        assert.equal(this.inbox, null)
    })

    test('message-no-inbox', function(done) {
        var lookup = []
        var notify
          , start = Date.now()
        this.reg.findToken = function(nick, foundCb, notFoundCb, errCb) {
            lookup.push(nick)
            if (nick == "N") {
                foundCb("T")
            } else {
                foundCb("T2")
            }
        }
        this.notifier.notify = function(nick, token, data) {
            notify = [nick, token, data]
        }
        request(this.app)
            .post('/message')
            .set('Content-Type', 'application/json')
            .send({nick: "N2", data: {"m": 1}, from_nick: "N", from_token: "T"})
            .expect('Content-Type', 'application/json; charset=utf-8')
            .expect({ok: true})
            .expect(200, function(err) {
                assert.deepEqual(lookup, ["N", "N2"])
                assert.deepEqual(notify.slice(0, 2), ["N2", "T2"])
                var data = notify[2]
                assert.equal(typeof(data._ephemeral), "number")
                assert.ok(data._ephemeral >= start)
                delete data._ephemeral
                assert.deepEqual(data, {"m": 1, _from:"N"})
                done(err)
            })
    })

    test('drain-not-there', function(done) {
        request(this.app)
            .post('/drain')
            .set('Content-Type', 'application/json')
             .send({nick: "N", token: "T", timestamp: 4000})
            .expect(404, done)
    })

})
