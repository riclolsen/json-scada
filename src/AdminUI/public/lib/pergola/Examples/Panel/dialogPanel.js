
//=============================   pergola examples - dialog panel   ==========================

/*
 * Copyright (C) 2009-2010  Dotuscomus - http://www.dotuscomus.com/
*/


var myPanel = new pergola.Panel("My Panel")
.build({
  type : "dialog",
  title : "DIALOG PANEL",
  x : 100,
  y : 100,
  width : 400,
  height : 360,
  okButton : {},                 // The "text" property of this object defaults to "OK"
  cancelButton : {},             // The "text" property of this object defaults to "Cancel"
  isOpen : true             // panels are mostly used for dialogs. Default is false
});




/*
 * For the okButton and cancelButton objects we can override the "text" and "marginRight" properties.
*/
