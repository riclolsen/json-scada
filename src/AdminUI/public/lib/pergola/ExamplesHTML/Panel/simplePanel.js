
//=============================   Pergola examples - simple panel   ==========================



var panel = new pergola.Panel()
.build({
  y : 50,
  title : "SIMPLE PANEL",
  isOpen : true,             // panels are mostly used for dialogs. Default is false
});



/*
 * The "type" prototype property of the Panel class is set to "dialog".
 * A panel of this type has a top bar and can have dialog buttons.
 *
 * The "type" property can be overridden with the value "basic".
 * A panel of this type is static, doesn't have dialog buttons,
 * and cannot be closed by the user.
*/