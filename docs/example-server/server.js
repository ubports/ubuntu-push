/*
  Push Notifications App Server Example
*/

var wire = require('./app').wire

var cfg = require('./config/config')
var mongoURL = 'mongodb://' + cfg.mongo_host + ':' + cfg.mongo_port + '/pushApp'

var MongoClient = require('mongodb').MongoClient

MongoClient.connect(mongoURL, cfg.mongo_opts, function(err, database) {
    if(err) throw err

    // wire appplication
    var app = wire(database, cfg)

    // log errors
    app.on('pushError', function(err, resp, body) {
        console.error('pushError', err, resp, body)
    })
    app.on('mongoError', function(err) {
        console.error('mongoError', err)
    })

    // connection ready => start app
    app.listen(cfg.listen_port)
    console.info("Listening on:", cfg.listen_port)
})
