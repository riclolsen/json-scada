
//=============================   Pergola examples - simple panel   ==========================


var g = $C({element : "g", transform : "translate(20 300)", "font-size" : 60, "font-weight" : "bold", appendTo : pergola.user});
$C({element : "text", textNode : "panel frame", appendTo : g});
$C({element : "text", x : 174, y : 48, textNode : "transparency", appendTo : g});


/*
 * Content for the panel's "contains" property (function or node).
*/
g = $C({element : "g", "font-size" : 60, "font-weight" : "bold"});
$C({element : "text", x : 174, y : 174, "font-size" : 20, "font-weight" : "bold", textNode : "opaque contents", appendTo : g});
$C({element : "circle", cx : 106, cy : 270, r : 60, fill : "red", appendTo : g});



var panel = new pergola.Panel("basic panel")
.build({
  type : "basic",
  x : 100,
  fill : "#FFFFF8",
  "fill-opacity": .96,
  "stroke-opacity" : .8,
  filter : "none",
  isOpen : true,             // panels are mostly used for dialogs. Default is false
  contains : g
});



/*
 * The "type" prototype property of the Panel class is set to "dialog".
 * A panel of this type has a top bar and can have buttons.
 *
 * The "type" property can be overridden with the value "basic".
 * A panel of this type is static, doesn't have buttons, and
 * cannot be closed by the user.
*/
