var assert = require('assert')
  ,  http = require('http')


var Notifier = require('../lib/notifier')

var cfg = {
    'app_id': 'app1',
    'expire_mins': 10,
    'retry_secs': 0.05,
    'retry_batch': 1,
    'happy_retry_secs': 0.02
}
  , cfg_batch2 = {
    'app_id': 'app1',
    'expire_mins': 10,
    'retry_secs': 0.05,
    'retry_batch': 2,
    'happy_retry_secs': 0.02
}

suite('Notifier', function() {
    setup(function(done) {
        var self = this
        self.s = http.createServer(function(req, resp) {
            self.s.emit(req.method, req, resp)
        })
        self.s.listen(0, 'localhost', function() {
            self.url = 'http://localhost:' + self.s.address().port
            done()
        })
    })

    teardown(function() {
        this.s.close()
    })

    test('happy-notify', function(done) {
        var b = ""
        this.s.on('POST', function(req, resp) {
            req.on('data', function(chunk) {
                b += chunk
            })
            req.on('end', function() {
                resp.writeHead(200, {"Content-Type": "application/json"})
                resp.end('{}')
            })
        })
        var n = new Notifier(this.url, cfg)
        var approxExpire = new Date
        approxExpire.setUTCMinutes(approxExpire.getUTCMinutes()+10)
        n.notify("N", "T", {m: 42}, function() {
            var reqObj = JSON.parse(b)
            var expireOn = Date.parse(reqObj.expire_on)
            delete reqObj.expire_on
            assert.ok(expireOn >= approxExpire)
            assert.deepEqual(reqObj, {
                "token": "T",
                "appid": "app1",
                "data": {"m": 42}
            })
            done()
        })
    })

    test('retry-notify', function(done) {
        var b = ""
        var fail = 1
        this.s.on('POST', function(req, resp) {
            if (fail) {
                fail--
                resp.writeHead(503, {"Content-Type": "application/json"})
                resp.end('')
                return
            }
            req.on('data', function(chunk) {
                b += chunk
            })
            req.on('end', function() {
                resp.writeHead(200, {"Content-Type": "application/json"})
                resp.end('{}')
            })
        })
        var n = new Notifier(this.url, cfg)
        var approxExpire = new Date
        approxExpire.setUTCMinutes(approxExpire.getUTCMinutes()+10)
        n.notify("N", "T", {m: 42}, function() {
            var reqObj = JSON.parse(b)
            var expireOn = Date.parse(reqObj.expire_on)
            delete reqObj.expire_on
            assert.ok(expireOn >= approxExpire)
            assert.deepEqual(reqObj, {
                "token": "T",
                "appid": "app1",
                "data": {"m": 42}
            })
            done()
        })
    })

    function flakyPOST(s, fail, tokens) {
        s.on('POST', function(req, resp) {
            var b = ""
            req.on('data', function(chunk) {
                b += chunk
            })
            req.on('end', function() {
                var reqObj = JSON.parse(b)
                if (fail) {
                    fail--
                    resp.writeHead(503, {"Content-Type": "application/json"})
                    resp.end('')
                    return
                }
                tokens[reqObj.token] = 1
                resp.writeHead(200, {"Content-Type": "application/json"})
                resp.end('{}')
            })
        })
    }

    test('retry-notify-2-retries', function(done) {
        var tokens = {}
        flakyPOST(this.s, 2, tokens)
        var n = new Notifier(this.url, cfg)
        function yay() {
            assert.deepEqual(tokens, {"T1": 1})
            done()
        }
        n.notify("N1", "T1", {m: 42}, yay)
    })

    test('retry-notify-2-batches', function(done) {
        var tokens = {}
        flakyPOST(this.s, 2, tokens)
        var n = new Notifier(this.url, cfg)
        var waiting = 2
        function yay() {
            waiting--
            if (waiting == 0) {
                assert.deepEqual(tokens, {"T1": 1, "T2": 1})
                done()
            }
        }
        n.notify("N1", "T1", {m: 42}, yay)
        n.notify("N2", "T2", {m: 42}, yay)
    })

    test('retry-notify-expired', function(done) {
        var tokens = {}
        flakyPOST(this.s, 2, tokens)
        var n = new Notifier(this.url, cfg_batch2)
        var waiting = 1
        function yay() {
            waiting--
            if (waiting == 0) {
                assert.deepEqual(tokens, {"T2": 1})
                done()
            }
        }
        n.notify("N1", "T1", {m: 42}, yay, new Date)
        n.notify("N2", "T2", {m: 42}, yay)
    })

    test('unknown-token-notify', function(done) {
        this.s.on('POST', function(req, resp) {
            resp.writeHead(400, {"Content-Type": "application/json"})
            resp.end('{"error": "unknown-token"}')
        })
        var n = new Notifier(this.url, cfg)
        n.on('unknownToken', function(nick, token) {
            assert.equal(nick, "N")
            assert.equal(token, "T")
            done()
        })
        n.notify("N", "T", {m: 42})
    })

    test('error-notify', function(done) {
        this.s.on('POST', function(req, resp) {
            resp.writeHead(500)
            resp.end('')
        })
        var n = new Notifier(this.url, cfg)
        n.on('pushError', function(err, resp, body) {
            assert.equal(resp.statusCode, 500)
            done()
        })
        n.notify("N", "T", {m: 42})
    })

})
