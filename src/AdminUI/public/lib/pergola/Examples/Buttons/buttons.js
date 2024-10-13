
//=============================   Pergola examples - buttons   ==========================



// Let's have a parent node first.
var g = $C({element : "g", transform : "translate(140 80)", appendTo : pergola.user});



var basicButton = new pergola.Button("Basic Button")
.build({
  parent : g,
  quickTip : {tip : "this is the basic, default button"},
  ev : "mouseup",
	target : legend.item1,
	fn : legend.hilight
});



var button = new pergola.Button()
.build({
  parent : g,
  x : 80,
  extra : {
  	rx : 3,
		transform : "rotate(-45)"
	},
  symbol : {
		symbol : pergola.symbols.arrow.large.left,
		x : 5,
    y : 5
	},
  quickTip : {tip : "this button is rotated, \nbut its symbol is straight"},
  ev : "mouseup",
	target : legend.item6,
	fn : legend.hilight
});



var buttonWithScaledSymbol = new pergola.Button()
.build({
  parent : g,
  x : 160,
  symbol : {
		symbol : pergola.symbols.arrow.large.left,
		attributes : {
			transform : "scale(.65 1.25)"
		}
	},
  quickTip : {tip : "this button has a scaled symbol"},
  ev : "mouseup",
	target : legend.item7,
	fn : legend.hilight
});



var button1 = new pergola.Button()
.build({
  parent : g,
  x : 240,
  size : "small",
  symbol : {
		symbol : pergola.symbols.arrow.small.up,
		attributes : {
			fill : "black"
		}
	},
  quickTip : {tip : "an object using a\nsymbol can override\nits \"fill\" property"}
});



var button2 = new pergola.Button("Button 2")
.build({
  parent : g,
  x : 320,
  symbol : {
		symbol : pergola.symbols.dot.large,
	},
  quickTip : {tip : "I'm normal with a normal symbol"},
  ev : "mouseup",
	target : legend.item8,
	fn : legend.hilight
});



var button3 = new pergola.Button("button 3")
.build({
  parent : g,
  x : 400,
  size : "small",
  symbol : {
  	symbol : pergola.symbols.dot.small,
  	attributes : {
			fill : "red"
		}
  },
  quickTip : {tip : "an object using a\nsymbol can override\nits \"fill\" property"}
});



var button4 = new pergola.Button("Button 4")
.build({
  parent : g,
  y : 100,
  width : 88,
  height : 28,
  extra : {
	  rx : 3
	},
  textFillInverse : "red",
  text : {
		x : 44,
		y : 36,
		textNode : "TALK",
		fill : "none",
		stroke : "#FFFFFF",
		"stroke-width" : 1.5,
		"font-family" : "'Arial Black', Arial",
		"font-size" : 22,
		"font-weight" : "bold",
		transform : "scale(1 .5)",
		"text-anchor" : "middle",
		"letter-spacing" : 2,
		kerning : 0,
		"pointer-events" : "none"
	},
  quickTip : {tip : [
		"you can place text and symbols together",
		"in a button. I don't have a symbol, but",
		"just to let you know..."
	]},
  ev : "mouseup",
	target : legend.item9,
	fn : legend.hilight
});



var button5 = new pergola.Button()
.build({
  parent : g,
  x : 150,
  y : 100,
  width : 12,
  height : 28,
  extra : {
	  rx : 2
	},
  symbol : {
		symbol : pergola.symbols.arrow.large.down,
		attributes : {
			transform : "scale(.65 1.5)",
    	fill : "#80C8FF"
		}
	},
  quickTip : {tip : "a scaled symbol"}
});



var roundButton = new pergola.Button()
.build({
  parent : g,
  x : 222,
  y : 106,
  width : 15,
  height : 15,
  extra : {
	  rx : 7.5,
	  "stroke-width" : 2.5
	},
  quickTip : {tip : "I'm really a rectangle"}
});



var tinyRoundButton = new pergola.Button()
.build({
  parent : g,
  x : 286,
  y : 110,
	shape : {
		element : "circle",
		cx : 4,
		cy : 4,
		r : 4
	},
  quickTip : {tip : "I'm really a circle"}
});



var button6 = new pergola.Button()
.build({
  parent : g,
  x : 350,
  y : 106,
  stroke : "lightsteelblue",
  extra : {
	  "stroke-width" : 3
	},
  symbol : {
  	symbol : pergola.symbols.arrow.large.left,
  	attributes : {
    	fill : "navy"
		}
  },
  quickTip : {tip : "an object using a\nsymbol can override\nits \"fill\" property"}
});



var monaLisa = new pergola.Button("Mona Lisa")
.build({
  parent : g,
  x : 440,
  y : 92,
  width : 59,
  height : 46,
  fill : "#F8F2DF",
  maskFill : "#F8EADF",
  stroke : "#F8EADF",
  image : {
		"xlink:href" : pergola.path + "lib/symbols/MonaLisa.jpg",
		width : 53,
  	height : 40,
  	x : 3,
    y : 3
	},
  extra : {
	  "stroke-width" : 4,
	  "stroke-dasharray" : "3,3",
	  "stroke-dashoffset" : 0
	},
  symbol : {
  	symbol : pergola.path + "lib/symbols/MonaLisa_negative.jpg",
  	width : 53,
		height : 40,
    x : 3,
    y : 3,
    opacity : .8
  },
  quickTip : {tip : "Thanks."},
  ev : "mouseup",
	target : legend.item10,
	fn : legend.hilight
});



var sillyButton = new pergola.Button("Silly Button");
sillyButton.build({
  parent : g,
  x : 1,
  y : 200,
  width : 22,
  height : 22,
  fill : "hotpink",
  stroke : "lightsteelblue",
  maskFill : "khaki",
  extra : {
	  rx : 6,
	  "stroke-width" : 2
	},
  quickTip : {tip : [
		"My visible name is " + sillyButton.name + " but my id is",
		sillyButton.id + ", I got that at the factory and if I",
		"didn't have a name my id would have been",
		"some XML_096E1D5C kind of thing  :("
	]},
  ev : "mouseup",
	target : legend.item5,
	fn : legend.hilight
});



var strangeButton = new pergola.Button("Strange Button");
strangeButton.build({
  parent : g,
  x : 80,
  y : 200,
  width : 48,
  height : 22,
  fill : "#4090D0",
  stroke : "lightsteelblue",
	maskFill : pergola.color.shade([100, 100, 100], 55),
  extra : {
	  rx : 24,
	  ry : 11,
	  "stroke-width" : 3,
	  "stroke-dasharray" : "28.5,28.5",
	  "stroke-linecap" : "round"
	},
  quickTip : {tip : "I'm " + strangeButton.name + ". My constructor\nfound \"stroke-dasharray\" and\n\"stroke-linecap\" in my properties."},
  ev : "mouseup",
	target : legend.item4,
	fn : legend.hilight
});



var shapeButton = new pergola.Button("shapeButton")
.build({
  parent : g,
  x : 190,
  y : 194,
  fill : "#FFB800",
  stroke : "lightsteelblue",
  maskFill : "lightsteelblue",
  shape : pergola.shapes.triangle,
  quickTip : {tip : "library shape"},
  ev : "mouseup",
	target : legend.item2,
	fn : legend.hilight
});



var shapeButton1 = new pergola.Button()
.build({
  parent : g,
  x : 272,
  y : 194,
  fill : "#509000",
  stroke : "lightsteelblue",
  maskFill : "white",
  shape : pergola.shapes.triangle,
  extra : {
		transform : "scale(2 .5)"
	},
	quickTip : {tip : "library shape with transformations"}
});



var shapeButton2 = new pergola.Button()
.build({
  parent : g,
  x : 272,
  y : 214.5,
  fill : "#FF5000",
  stroke : "lightsteelblue",
	maskFill : "white",
  shape : pergola.shapes.triangle,
  extra : {
	  transform : "scale(2 .5) rotate(180)"
	},
  quickTip : {tip : "library shape with transformations"}
});



var amoebaButton = new pergola.Button("Amœba Button")
.build({
  parent : g,
  x : 380,
  y : 194,
  fill : "#F4F4F4",
  stroke : "lightsteelblue",
  maskFill : "#D8E8D8",
  shape : {
		element : "path",
		d : "M16.14,2.53C23.64 12.1 29 9 33.3 14.23C37.1 18.85	29.34 22.78 35.7 28.6C39.79 32.35 36.08 39.69 28.12 40.31C-3.59 42.78 8.98 27.12 5.5 21.71C-5.96 3.87 11.69 -3.14 16.14 2.52z"
	},
  quickTip : {tip : "Amœba Button\nUser defined shape"},
  ev : "mouseup",
	target : legend.item3,
	fn : legend.hilight,
	contextMenuItems : {
	  changeToRed : {
	    string : "Change To Red",
	    fn : function (evt) {alert(this.name)}
	  },
	  changetoBlue : {
	    string : "Change To Blue",
	    fn : function (evt) {alert("")}
	  },
	  rotate : {
	    string : "Rotate 45°",
	    fn : function (evt) {alert("")},
	    separator : new pergola.Separator()
	  },
	  remove : {
	    string : "Delete",
	    fn : function () {p.parentNode.removeChild(p);}
	  }
	}
});


