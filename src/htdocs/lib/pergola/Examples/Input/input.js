
//=============================   Pergola examples - Input   ==========================




// A parent node
var g = $C({element : "g", transform : "translate(100 50)", appendTo : pergola.user});


var myObject = {
  r : 255,
  g : 128,
  b : 0,
  circle : $C({element : "circle", cx : 130, cy : 40, r : 24, fill : "rgb(255, 128, 0)", stroke : "#A0A0A0", "stroke-width" : 2, appendTo : g}),
  changeColor : function () {
    this.circle.setAttributeNS(null, "fill", "rgb(" + this.r + "," + this.g + "," + this.b + ")");
  },
  input : {
    r : new pergola.Input(),
    g : new pergola.Input(),
    b : new pergola.Input()
  }
};



myObject.input.r.build({
  owner : myObject,
  parent : g,
  callback : {keypress : pergola.Key.keypressPosInt},
  caption : {
    textNode : "R",
  },
  hasButtons : true,
  max : 255,
  min : 0,
  maxLength : 3,
  value : myObject.r + "",
  allowEmpty : false,
  target : myObject,
  propName : "r",
  fn : "changeColor",
  realTime : true
});

myObject.input.g.build({
  owner : myObject,
  parent : g,
  callback : {keypress : pergola.Key.keypressPosInt},
  y : 30,
  caption : {
    textNode : "G",
  },
  hasButtons : true,
  max : 255,
  min : 0,
  maxLength : 3,
  value : myObject.g + "",
  allowEmpty : false,
  target : myObject,
  propName : "g",
  fn : "changeColor",
  realTime : true
});
myObject.input.b.build({
  owner : myObject,
  parent : g,
  callback : {keypress : pergola.Key.keypressPosInt},
  y : 60,
  caption : {
    textNode : "B",
  },
  hasButtons : true,
  max : 255,
  min : 0,
  maxLength : 3,
  value : myObject.b + "",
  allowEmpty : false,
  target : myObject,
  propName : "b",
  fn : "changeColor",
  realTime : true
});
