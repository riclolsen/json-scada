
//=============================   Pergola examples - simple panel   ==========================



var panel = new pergola.Panel("simple panel")
.build({
  title : "SIMPLE PANEL",
  isOpen : true             // panels are mostly used for dialogs. Default is false
});



/*
 * The "type" prototype property of the Panel class is set to "dialog".
 * A panel of this type has a top bar and can have buttons.
 *
 * The "type" property can be overridden with the value "basic".
 * A basic panel is static, doesn't have buttons, and
 * cannot be closed by the user.
*/
