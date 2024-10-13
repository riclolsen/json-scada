
//==========================   Pergola examples - panel with combobox   ==========================




/*
 * comboboxes are usally found in dialogs, and selections ara validated by a doubleclick or the
 * dialog's OK button, while the cancel button cancels the selection. For this example we build
 * a dialog panel and have it use the Layout class to create the combobox instance, referenced
 * by the "combobox" property of the panel. The list "myList" used by the combobox is defined
 * in the "list" file.
 *
 * To see how to instantiate a standalone combobox, see the standalone_combo.svg example.
*/


var myPanel = new pergola.Panel("my panel")
.build({
  title : "PANEL WITH COMBO BOX",
  x : 150,
  y : 50,
  okButton : {
    text : "OK"                  // this is the default value. Redundant
  },
  cancelButton : {
    text : "Cancel"              // this is the default value. Redundant
  },
  fn : "listItemFunction",
  isOpen : true,             // panels are mostly used for dialogs. Default is false
  layout : {
    type : "combobox",           // the name of a prototype method of the Layout class
    x : 100,
    y : 60,
    width : 240,
    height : 180,
    list : myList
  },
// For this demo we want the panel to remain open. Let's override its prototype method closeControl()
  close : function (evt) {
    if (evt.target == this.closeBtn.button || evt.target == this.cancel.button) $M("In this demo the close() prototype method \nwas overridden to show this message and \nleave the panel open.", {x : 240, y : 160});
  }
});


/*
 * a simple user function for combobox list items
*/
myPanel.listItemFunction = function (evt) {
  var handle = this.comboBox.selectionHandle;
  if (handle) {
    for (var m in myList) {
      if (handle.string == myList[m]) $M(myMessages[m], {x : 300, y : 220});
    }
  }
}



