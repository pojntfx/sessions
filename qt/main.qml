import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as Controls
import org.kde.kirigami as Kirigami

Kirigami.ApplicationWindow {
    id: root

    width: 360
    height: 360

    title: "Sessions"

    pageStack.initialPage: Kirigami.Page {

        ColumnLayout {
            anchors.fill: parent

            Controls.Label {
                id: analogTimeLabel

                text: "05:00"

                Layout.alignment: Qt.AlignCenter
            }

            Controls.Button {
                id: minusButton

                text: "Remove 30 seconds"

                Layout.alignment: Qt.AlignCenter
            }

            Controls.Button {
                id: actionButton

                text: "Start Timer"

                Layout.alignment: Qt.AlignCenter
            }

            Controls.Button {
                id: plusButton

                text: "Add 30 seconds"

                Layout.alignment: Qt.AlignCenter
            }
        }
    }
}
