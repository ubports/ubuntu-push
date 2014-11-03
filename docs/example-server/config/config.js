module.exports = config = {
    "name" : "pushAppServer"
    ,"app_id" : "appEx"
    ,"listen_port" : 8000
    ,"mongo_host" : "localhost"
    ,"mongo_port" : 27017
    ,"mongo_opts" : {}
    ,"push_url": "https://push.ubuntu.com"
    ,"retry_batch": 5
    ,"retry_secs" : 30
    ,"happy_retry_secs": 5
    ,"expire_mins": 120
    ,"no_inbox": true
    ,"play_notify_form": true
}
