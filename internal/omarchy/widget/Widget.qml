import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// pigeon's icon in Omarchy's bar, put there by pigeon (Settings → Bar icon).
// A click drops down its panel under the bar, where the click was; a right
// click opens its window. pigeon is started if it isn't running.
BarWidget {
  id: root
  moduleName: "usemagpie.pigeon"

  // written in by pigeon when it puts the widget here
  readonly property string pigeon: "__MAGPIE__"

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: String.fromCodePoint(0xf15c6) // nf-md-bird
    tooltipText: "pigeon"
    onPressed: function(b) {
      Quickshell.execDetached(b === Qt.RightButton ? [root.magpie, "app"] : [root.magpie, "panel"])
    }
  }
}
