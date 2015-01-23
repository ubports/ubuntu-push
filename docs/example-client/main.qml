import QtQuick 2.0
import Qt.labs.settings 1.0
import Ubuntu.Components 0.1
import Ubuntu.Components.ListItems 0.1 as ListItem
import Ubuntu.PushNotifications 0.1
import "components"

MainView {
    id: "mainView"
    // objectName for functional testing purposes (autopilot-qt5)
    objectName: "mainView"

    // Note! applicationName needs to match the "name" field of the click manifest
    applicationName: "com.ubuntu.developer.ralsina.hello"

    automaticOrientation: true
    useDeprecatedToolbar: false

    width: units.gu(100)
    height: units.gu(75)

    Settings {
        property alias nick: chatClient.nick
        property alias nickText: nickEdit.text
        property alias nickPlaceholder: nickEdit.placeholderText
        property alias nickEnabled: nickEdit.enabled
    }

    states: [
        State {
            name: "no-push-token"
            when: (pushClient.token == "")
            PropertyChanges { target: nickEdit; readOnly: true}
            PropertyChanges { target: nickEdit; focus: true}
            PropertyChanges { target: messageEdit; enabled: false}
            PropertyChanges { target: loginButton; enabled: false}
            PropertyChanges { target: loginButton; text: "Login"}
        },
        State {
            name: "push-token-not-registered"
            when: ((pushClient.token != "") && (chatClient.registered == false))
            PropertyChanges { target: nickEdit; readOnly: false}
            PropertyChanges { target: nickEdit; text: ""}
            PropertyChanges { target: nickEdit; focus: true}
            PropertyChanges { target: messageEdit; enabled: false}
            PropertyChanges { target: loginButton; enabled: true}
            PropertyChanges { target: loginButton; text: "Login"}
        },
        State {
            name: "registered"
            when: ((pushClient.token != "") && (chatClient.registered == true))
            PropertyChanges { target: nickEdit; readOnly: true}
            PropertyChanges { target: nickEdit; text: "Your nick is " + chatClient.nick}
            PropertyChanges { target: messageEdit; focus: true}
            PropertyChanges { target: messageEdit; enabled: true}
            PropertyChanges { target: loginButton; enabled: true}
            PropertyChanges { target: loginButton; text: "Logout"}
        }
    ]

    state: "no-push-token"

    ChatClient {
        id: chatClient
        onError: {messageList.handle_error(msg)}
        token: pushClient.token
    }

    PushClient {
        id: pushClient
        Component.onCompleted: {
            notificationsChanged.connect(messageList.handle_notifications)
            error.connect(messageList.handle_error)
            onTokenChanged: {
                console.log("foooooo")
            }
        }
        appId: "com.ubuntu.developer.ralsina.hello_hello"

    }

    TextField {
        id: nickEdit
        placeholderText: "Your nickname"
        inputMethodHints: Qt.ImhNoAutoUppercase | Qt.ImhNoPredictiveText | Qt.ImhPreferLowercase
        anchors.left: parent.left
        anchors.right: loginButton.left
        anchors.top: parent.top
        anchors.leftMargin: units.gu(.5)
        anchors.rightMargin: units.gu(1)
        anchors.topMargin: units.gu(.5)
        onAccepted: { loginButton.clicked() }
    }

    Button {
        id: loginButton
        anchors.top: nickEdit.top
        anchors.right: parent.right
        anchors.rightMargin: units.gu(.5)
        onClicked: {
            if (chatClient.nick) { // logout
                chatClient.nick = ""
            } else { // login
                chatClient.nick = nickEdit.text
            }
        }
    }

    TextField {
        id: messageEdit
        inputMethodHints: Qt.ImhNoAutoUppercase | Qt.ImhNoPredictiveText | Qt.ImhPreferLowercase
        anchors.right: parent.right
        anchors.left: parent.left
        anchors.top: nickEdit.bottom
        anchors.topMargin: units.gu(1)
        anchors.rightMargin: units.gu(.5)
        anchors.leftMargin: units.gu(.5)
        placeholderText: "Your message"
        onAccepted: {
            console.log("sending " + text)
            var idx = text.indexOf(":")
            var nick_to = text.substring(0, idx).trim()
            var msg = text.substring(idx+1, 9999).trim()
            var i = {
                "from" :  chatClient.nick,
                "to" :  nick_to,
                "message" : msg
            }
            var o = {
                enabled: annoyingSwitch.checked,
                persist: persistSwitch.checked,
                popup: popupSwitch.checked,
                sound: soundSwitch.checked,
                vibrate: vibrateSwitch.checked,
                counter: counterSlider.value
            }
            chatClient.sendMessage(i, o)
            i["type"] = "sent"
            messagesModel.insert(0, i)
            text = ""
        }
    }
    ListModel {
        id: messagesModel
        ListElement {
            from: ""
            to: ""
            type: "info"
            message: "Register by typing your nick and clicking Login."
        }
        ListElement {
            from: ""
            to: ""
            type: "info"
            message: "Send messages in the form \"destination: hello\""
        }
        ListElement {
            from: ""
            to: ""
            type: "info"
            message: "Slide from the bottom to control notification behaviour."
        }
    }

    UbuntuShape {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: notificationSettings.bottom
        anchors.top: messageEdit.bottom
        anchors.topMargin: units.gu(1)
        ListView {
            id: messageList
            model: messagesModel
            anchors.fill: parent
            delegate: Rectangle {
                MouseArea {
                    anchors.fill: parent
                    onClicked: {
                        if (from != "") {
                            messageEdit.text = from + ": "
                            messageEdit.focus = true
                        }
                    }
                }
                height: label.height + units.gu(2)
                width: parent.width
                Rectangle {
                    color: {
                        "info": "#B5EBB9",
                        "received" : "#A2CFA5",
                        "sent" : "#FFF9C8",
                        "error" : "#FF4867"}[type]
                    height: label.height + units.gu(1)
                    anchors.fill: parent
                    radius: 5
                    anchors.margins: units.gu(.5)
                    Text {
                        id: label
                        text: "<b>" + ((type=="sent")?to:from) + ":</b> " + message
                        wrapMode: Text.Wrap
                        width: parent.width - units.gu(1)
                        x: units.gu(.5)
                        y: units.gu(.5)
                        horizontalAlignment: (type=="sent")?Text.AlignRight:Text.AlignLeft
                    }
                }
            }

            function handle_error(error) {
                messagesModel.insert(0, {
                     "from" :  "",
                     "to" :  "",
                     "type" :  "error",
                     "message" : "<b>ERROR: " + error + "</b>"
                })
            }

            function handle_notifications(list) {
                list.forEach(function(notification) {
                    var item = JSON.parse(notification)
                    item["type"] = "received"
                    messagesModel.insert(0, item)
                })
            }
        }
    }
    Panel {
        id: notificationSettings
        anchors {
            left: parent.left
            right: parent.right
            bottom: parent.bottom
        }
        height: item1.height * 9
        UbuntuShape {
            anchors.fill: parent
            color: Theme.palette.normal.overlay
            Column {
                id: settingsColumn
                anchors.fill: parent
                ListItem.Header {
                    text: "<b>Notification Settings</b>"
                }
                ListItem.Standard {
                    id: item1
                    text: "Enable Notifications"
                    control: Switch {
                        id: annoyingSwitch
                        checked: true
                    }
                }
                ListItem.Standard {
                    text: "Enable Popup"
                    enabled: annoyingSwitch.checked
                    control: Switch {
                        id: popupSwitch
                        checked: true
                    }
                }
                ListItem.Standard {
                    text: "Persistent"
                    enabled: annoyingSwitch.checked
                    control: Switch {
                        id: persistSwitch
                        checked: true
                    }
                }
                ListItem.Standard {
                    text: "Make Sound"
                    enabled: annoyingSwitch.checked
                    control: Switch {
                        id: soundSwitch
                        checked: true
                    }
                }
                ListItem.Standard {
                    text: "Vibrate"
                    enabled: annoyingSwitch.checked
                    control: Switch {
                        id: vibrateSwitch
                        checked: true
                    }
                }
                ListItem.Standard {
                    text: "Counter Value"
                    enabled: annoyingSwitch.checked
                    control: Slider {
                        id: counterSlider
                        value: 42
                    }
                }
                Button {
                    text: "Set Counter Via Plugin"
                    onClicked: { pushClient.count = counterSlider.value; }
                }
                Button {
                    text: "Clear Persistent Notifications"
                    onClicked: { pushClient.clearPersistent([]); }
                }
            }
        }
    }
}
