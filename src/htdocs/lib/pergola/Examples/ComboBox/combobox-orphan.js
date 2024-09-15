
//==========================   Pergola examples - standalone combobox   ==========================



/*
 * In the combobox.svg example we had the Layout class build the combox instance for the dialog panel.
 * But we can also build a standalone combobox, which will be independent of any system logic,
 * by instantiating directly the ComboBox class.
*/


myComboBox = new pergola.ComboBox("my combobox")
.build({
  x : 200,
  y : 180,
  list : myList,
  fn : function (evt) {
    for (var m in myList) if (this.selection == myList[m]) $M(myMessages[m]);
  }
});
