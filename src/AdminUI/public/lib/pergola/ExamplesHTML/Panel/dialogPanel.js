
//=============================   pergola examples - dialog panel   ==========================




var myPanel = new pergola.Panel("My Panel")
.build({
  title : "DIALOG PANEL",
  x : 100,
  y : 50,
  width : 400,
  height : 300,
  okButton : {},                 // The "text" property of this object defaults to "OK"
  cancelButton : {},             // The "text" property of this object defaults to "Cancel"
  isOpen : true                  // panels are mostly used for dialogs. Default is false
});




/*
 * For the okButton and cancelButton objects you can override the "text" and "marginRight" properties.
*/