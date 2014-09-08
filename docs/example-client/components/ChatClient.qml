import QtQuick 2.0
import Ubuntu.Components 0.1

Item {
    property string nick
    property string token
    property bool registered: false
    signal error (string msg)
    onNickChanged: {
        if (nick) {
            register()
        } else {
            registered = false
        }
    }
    onTokenChanged: {register()}
    function register() {
        console.log("registering ", nick, token);
        if (nick && token) {
            var req = new XMLHttpRequest();
            req.open("post", "http://direct.ralsina.me:8001/register", true);
            req.setRequestHeader("Content-type", "application/json");
            req.onreadystatechange = function() {//Call a function when the state changes.
                if(req.readyState == 4) {
                    if (req.status == 200) {
                        registered = true;
                    } else {
                        error(JSON.parse(req.responseText)["error"]);
                    }
                }
            }
            req.send(JSON.stringify({
                "nick" : nick.toLowerCase(),
                "token": token
            }))
        }
    }

    /* options is of the form:
      {
          enabled: false,
          persist: false,
          popup: false,
          sound: "buzz.mp3",
          vibrate: false,
          counter: 5
      }
    */
    function sendMessage(message, options) {
        var to_nick = message["to"]
        var data = {
            "from_nick": nick.toLowerCase(),
            "from_token": token,
            "nick": to_nick.toLowerCase(),
            "data": {
                "message": message,
                "notification": {}
            }
        }
        if (options["enabled"]) {
            data["data"]["notification"] = {
                "card": {
                    "summary": nick + " says: " + message["message"],
                    "body": "",
                    "popup": options["popup"],
                    "persist": options["persist"],
                    "actions": ["appid://com.ubuntu.developer.ralsina.hello/hello/current-user-version"]
                }
            }
            if (options["sound"]) {
                data["data"]["notification"]["sound"] = options["sound"]
            }
            if (options["vibrate"]) {
                data["data"]["notification"]["vibrate"] = {
                    "duration": 200
                }
            }
            if (options["counter"]) {
                data["data"]["notification"]["emblem-counter"] = {
                    "count": Math.floor(options["counter"]),
                    "visible": true
                }
            }
        }
        var req = new XMLHttpRequest();
        req.open("post", "http://direct.ralsina.me:8001/message", true);
        req.setRequestHeader("Content-type", "application/json");
        req.onreadystatechange = function() {//Call a function when the state changes.
            if(req.readyState == 4) {
                if (req.status == 200) {
                    registered = true;
                } else {
                    error(JSON.parse(req.responseText)["error"]);
                }
            }
        }
        req.send(JSON.stringify(data))
    }
}
