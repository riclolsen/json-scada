
//======================   Pergola examples - colorpicker and ColorBoxSelector   ========================



/*
 * Pergola builds an instance of the ColorPicker class, pergola.colorpicker, which can be used
 * by any object. The object initializes the colorpicker by invoking its init() method which
 * expects one object as argument with the "user" and "color" properties set (see documentation).
 *
 * In this example the object is an instance of the ColorBoxSelector class, which is designed
 * specifically to use the colorpicker.
*/

var g = $C({element : "g", transform : "translate(100 50)", appendTo : pergola.user});

var mySelector = new pergola.ColorBoxSelector()
.build({
  parent : g,
  fill : "#FFFF80",
  caption : {
    y : 15,
    textNode : "Select color",
  },
//  target : pergola.background,
  fn : function () {
    var cp = pergola.colorpicker;

    cp.user.fill = cp.color;
    cp.user.rect.setAttributeNS(null, "fill", cp.color);
// example : change the background color
//    cp.user.target.fill = cp.color;
//    cp.user.target.rect.setAttributeNS(null, "fill", cp.color);
  }
});


/*
 * You can create other ColorPicker instances but only one instance at a time can be used.
*/
