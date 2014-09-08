/* nick -> token registry */

function Registry(db) {
    this.db = db
}

Registry.prototype.findToken = function(nick, foundCb, notFoundCb, errCb) {
    var self = this
    self.db.collection('registry').findOne({_id: nick}, function(err, doc) {
        if (err) {
            errCb(err)
            return
        }
        if (doc == null) {
            notFoundCb()
            return
        }
        foundCb(doc.token)
    })
}

Registry.prototype.insertToken = function(nick, token, doneCb, dupCb, errCb) {
    var self = this
    doc = {_id: nick, token: token}
    self.db.collection('registry').insert(doc, function(err) {
        if (!err) {
            doneCb()
        } else {
            if (err.code == 11000) { // dup
                self.findToken(nick, function(token2) {
                    if (token == token2) {
                        // same, idempotent
                        doneCb()
                        return
                    }
                    dupCb()
                }, function() {
                    // not found, try again
                    self.insertToken(nick, token, doneCb, dupCb, errCb)
                }, function(err) {
                    errCb(err)
                })
                return
            }
            errCb(err)
        }
    })
}

Registry.prototype.removeToken = function(nick, token, doneCb, errCb) {
    var self = this
    doc = {_id: nick, token: token}
    self.db.collection('registry').remove(doc, function(err) {
        if (err) {
            errCb(err)
            return
        }
        doneCb()
    })
}

module.exports = Registry
