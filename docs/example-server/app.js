var express = require('express')
  , bodyParser = require('body-parser')

var Registry = require('./lib/registry')
  , Inbox = require('./lib/inbox')
  , Notifier = require('./lib/notifier')

function wire(db, cfg) {
    var reg, inbox, notifier, app

    // control whether not to have persistent inboxes
    var no_inbox = cfg.no_inbox

    app = express()

    reg = new Registry(db)
    if (!no_inbox) {
        inbox = new Inbox(db)
    } else {
        inbox = null
    }
    var push_url = process.env.PUSH_URL || cfg.push_url
    notifier = new Notifier(push_url, cfg)
    notifier.on('unknownToken', function(nick, token) {
        reg.removeToken(nick, token, function() {},
                        function(err) {
                            app.emit('mongoError', err)
                        })
    })
    notifier.on('pushError', function(err, resp, body) {
        app.emit('pushError', err, resp, body)
    })

    function unavailable(resp, err) {
        app.emit('mongoError', err)
        var ctype = resp.get('Content-Type')
        if (ctype&&ctype.substr(0,10) == 'text/plain') {
            resp.send(503, "db is hopefully only momentarily :(\n")
        } else {
            resp.json(503, {error: "unavailable"})
        }
    }

    app.get("/_check", function(req, resp) {
        db.command({ping: 1}, function(err, res) {
            if(!err) {
                resp.json({ok: true})
            } else {
                unavailable(resp, err)
            }
        })
    })

    app.use(bodyParser.json())
    app.use('/play-notify-form', bodyParser.urlencoded({extended: false}))
    app.use(function(err, req, resp, next) {
        resp.json(err.status, {error: "invalid", message: err.message})
    })

    app.get("/", function(req, resp) {
        if (!cfg.play_notify_form) {
            resp.sendfile(__dirname + '/index.html')
        } else {
            resp.sendfile(__dirname + '/notify-form.html')
        }
    })

    // NB: simplified, minimal identity and auth piggybacking on push tokens

    /*
      POST /register let's register a pair nick, token taking a JSON obj:
      { "nick": string, "token": token-string }
    */
    app.post("/register", function(req, resp) {
        if(typeof(req.body.token) != "string" ||
           typeof(req.body.nick) != "string" ||
           req.body.token == "" || req.body.nick == "") {
            resp.json(400, {"error": "invalid"})
            return
        }
        reg.insertToken(req.body.nick, req.body.token, function() {
            resp.json({ok: true})
        }, function() {
            resp.json(400, {"error": "dup"})
        }, function(err) {
            unavailable(resp, err)
        })
    })

    function checkToken(nick, token, okCb, resp) {
        function bad() {
            resp.json(401, {error: "unauthorized"})
        }
        reg.findToken(nick, function(nickToken) {
            if (nickToken == token) {
                okCb()
                return
            }
            bad()
        }, bad, function(err) {
            unavailable(resp, err)
        })
    }

    /* doNotify

       ephemeral is true: message not put in the inbox, _ephemeral flag added

       ephemeral is false: message put in inbox, with added unix _timestamp and
       increasing _serial

     */
    function doNotify(ephemeral, nick, data, okCb, unknownNickCb, resp) {
        function notify(token, data) {
            notifier.notify(nick, token, data)
            okCb()
        }
        reg.findToken(nick, function(token) {
            if (ephemeral||no_inbox) {
                data._ephemeral = Date.now()
                notify(token, data)
            } else {
                inbox.pushMessage(token, data, function(msg) {
                    notify(token, msg)
                }, function(err) {
                    unavailable(resp, err)
                })
            }
        }, function() { // not found
            unknownNickCb()
        }, function(err) {
            unavailable(resp, err)
        })
    }

    /*
      POST /message let's send a message to nick taking a JSON obj:
      { "nick": string, "data": data,  "from_nick": string, "from_token": string}
    */
    app.post("/message", function(req, resp) {
        if (!req.body.data||!req.body.nick||!req.body.from_token||!req.body.from_nick) {
            resp.json(400, {"error": "invalid"})
            return
        }
        checkToken(req.body.from_nick, req.body.from_token, function() {
            var data = req.body.data
            data._from = req.body.from_nick
            doNotify(false, req.body.nick, data, function() {
                resp.json({ok: true})
            }, function() { // not found
                resp.json(400, {"error": "unknown-nick"})
            }, resp)
        }, resp)
    })

    if (!no_inbox) { // /drain supported only if no_inbox false
        /*
          POST /drain let's get pending messages for nick+token:
          it removes messages older than timestamp and return newer ones
          { "nick": string, "token": string, "timestamp": unix-timestamp }
        */
        app.post("/drain", function(req, resp) {
            if(!req.body.token||!req.body.nick||
               typeof(req.body.timestamp) != "number") {
                resp.json(400, {"error": "invalid"})
                return
            }
            checkToken(req.body.nick, req.body.token, function() {
                inbox.drain(req.body.token, req.body.timestamp, function(msgs) {
                    resp.json(200, {ok: true, msgs: msgs})
                }, function(err) {
                    unavailable(resp, err)
                })
            }, resp)
        })
    }

    /*
      Form /play-notify-form
      messages sent through the form are ephemeral, just transmitted through PN,
      without being pushed into the inbox
    */
    if (cfg.play_notify_form) {
        app.post("/play-notify-form", function(req, resp) {
            if (!req.body.message||!req.body.nick) {
                resp.redirect("/?error=invalid or empty fields in form")
                return
            }
            var data = {
                "message": {
                    "from": "website",
                    "message": req.body.message,
                    "to": req.body.nick.toLowerCase()
                },
                "notification": {
                }
            }

            if (req.body.enable) {
                var card = {
                    "summary": "The website says:",
                    "body": req.body.message,
                    "actions": ["appid://com.ubuntu.developer.ralsina.hello/hello/current-user-version"]
                }
                if (req.body.popup) {card["popup"] = true}
                if (req.body.persist) {card["persist"] = true}
                data["notification"]["card"] = card
                if (req.body.sound) {data["notification"]["sound"] = true}
                if (req.body.vibrate) {data["notification"]["vibrate"] = {"duration": 200}}
                if (req.body.counter) {data["notification"]["emblem-counter"] = {
                    "count": Math.floor(req.body.counter),
                    "visible": true
                }}
            }
            doNotify(true, req.body.nick, data, function() {
                resp.redirect("/")
            }, function() { // not found
                resp.redirect("/?error=unknown nick")
            }, resp)
        })
    }

    // for testing
    app.set('_reg', reg)
    app.set('_inbox', inbox)
    app.set('_notifier', notifier)
    return app
}

exports.wire = wire
