/* Notifier sends notifications, with some retry support */

var request = require('request')
  , url = require('url')
  , EventEmitter = require('events').EventEmitter
  , util = require('util')


function Notifier(baseURL, cfg) {
    this.baseURL = baseURL
    this.cfg = cfg
    this._retrier = null
    this._retryInterval = 0
    this._q = []
}

util.inherits(Notifier, EventEmitter)

Notifier.prototype._retry = function(nick, token, data, cb, expireOn) {
    var self = this
    self._q.push([nick, token, data, cb, expireOn])
    if (!self._retrier || self._retryInterval == self.cfg.happy_retry_secs) {
        clearTimeout(self._retrier)
        self._retryInterval = self.cfg.retry_secs
        self._retrier = setTimeout(function() { self._doRetry() }, 1000*self._retryInterval)
    }
}

Notifier.prototype._doRetry = function() {
    var self = this
    self._retryInterval = 0
    self._retrier = null
    var i = 0
    while (self._q.length > 0) {
        var toRetry = self._q.shift()
        if (new Date() > toRetry[4]) { // expired
            continue
        }
        self.notify(toRetry[0], toRetry[1], toRetry[2], toRetry[3], toRetry[4])
        i++
        if (i >= self.cfg.retry_batch) {
            break
        }
    }
    if (self._q.length) {
        self._retryInterval = self.cfg.happy_retry_secs
        self._retrier = setTimeout(function() { self._doRetry() }, 1000*self._retryInterval)
    }
}

Notifier.prototype.unknownToken = function(nick, token) {
    this.emit('unknownToken', nick, token)
}


Notifier.prototype.pushError = function(err, resp, body) {
    this.emit('pushError', err, resp, body)
}

Notifier.prototype.notify = function(nick, token, data, cb, expireOn) {
    var self = this
    if (!expireOn) {
        expireOn = new Date
        expireOn.setUTCMinutes(expireOn.getUTCMinutes() + self.cfg.expire_mins)
    }
    var unicast = {
        'appid': self.cfg.app_id,
        'expire_on': expireOn.toISOString(),
        'token': token,
        'data': data
    }
    request.post(url.resolve(self.baseURL, 'notify'), {json: unicast}, function(error, resp, body) {
        if (!error) {
            if (resp.statusCode == 200) {
                if (cb) cb()
                return
            } else if (resp.statusCode > 500) {
                self._retry(nick, token, data, cb, expireOn)
                return
            } else if (resp.statusCode == 400 && body.error == "unknown-token") {
                self.unknownToken(nick, token)
                return
            }
        }
        self.pushError(error, resp, body)
    })
}

module.exports = Notifier
