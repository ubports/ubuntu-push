/* token -> token inbox */

function Inbox(db) {
    this.db = db
}

Inbox.prototype.pushMessage = function(token, msg, doneCb, errCb) {
    if (!msg._timestamp) {
        var now = Date.now()
        msg._timestamp = now
    }
    this.db.collection('inbox').findAndModify({_id: token}, null, {
        $push: {msgs: msg},
        $inc: {serial: 1},
    }, {upsert: true, new: true, fields: {serial: 1}}, function(err, doc) {
        if (err) {
            errCb(err)
            return
        }
        msg._serial = doc.serial
        doneCb(msg)
    })
}

Inbox.prototype.drain = function(token, timestamp, doneCb, errCb) {
    this.db.collection('inbox').findAndModify({_id: token}, null, {
        $pull: {msgs: {_timestamp: {$lt: timestamp}}}
    }, {new: true}, function(err, doc) {
        if (err) {
            errCb(err)
            return
        }
        if (!doc) {
            doneCb([])
            return
        }
        var serial = doc.serial
        var msgs = doc.msgs
        for (var i = msgs.length-1; i >= 0; i--) {
            msgs[i].serial = serial
            serial--
        }
        doneCb(msgs)
    })
}

module.exports = Inbox
