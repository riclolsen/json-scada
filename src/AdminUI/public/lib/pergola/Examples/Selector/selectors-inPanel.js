
//=======================   Pergola examples - popup list selector   =====================



var shirtsPanel = new pergola.Panel()
.build({
  type : "dialog",
  title : "T SHIRTS",
  x : 200,
  y : 100,
  width : 440,
  height : 330,
  isOpen : true
});





var shirt = {container : $C({element : "g", transform : "translate(270 110)", appendTo : shirtsPanel.container})};

pergola.extend(shirt, {
	x : 18.5,
  y : 14,
	shape : $C({element : "path", d : "M0 0h14q4.5 4.5,9 0h14v9h-5v24h-27v-24h-5z", transform : "scale(2)", fill : "#0040C0", appendTo : shirt.container}),
	text : $C({element : "text", x : 37, y : 28, "font-size" : "10px", "font-weight" : "bold", "text-anchor" : "middle", kerning : 0, fill : "white", textNode : "PERGOLA", appendTo : shirt.container}),
	colors : ["#FF0000", "#FF8000", "#0040C0", "#D000D0", "#00A040"],
	colorNames : ["Red", "Orange", "Blue", "Purple", "Green"],
	sizes : ["Small", "Medium", "Large", "Extra large"],
	qtyValues : ["10", "50", "100", "500", "1000"],
	quantity : 10, 
	size : "Medium",
	color : "Blue",

  setSize : function (i) {
  	this.size = this.sizes[i]
    this.shape.setAttributeNS(null, "transform", "scale(" + ++i + ")");
		pergola.attribute(this.text, {"font-size" : 5 * i, x : this.x * i, y : this.y * i});
  },

  setColor : function (i) {
  	this.color = this.colors[i];
    this.shape.setAttributeNS(null, "fill", this.color);
  },

  setQty : function (i) {
  	this.quantity = this.qtyValues[i];
  }
});

new pergola.Selector()
.build({
	owner : shirt,
	parent : shirtsPanel.container,
	x : 60,
	y : 110,
  width : 180,
	list : shirt.sizes,
	index : 1,
	caption : {
    y : 15,
    textNode : "Size"
  },
  fn : function (evt) {
		this.owner.setSize(this.index);
	}
});

new pergola.Selector()
.build({
	owner : shirt,
	parent : shirtsPanel.container,
	x : 60,
	y : 160,
  width : 180,
	list : shirt.colorNames,
	index : 2,
	caption : {
    y : 15,
    textNode : "Color"
  },
  fn : function (evt) {
		this.owner.setColor(this.index);
	}
});

new pergola.Selector()
.build({
	owner : shirt,
	parent : shirtsPanel.container,
	x : 60,
	y : 210,
	width : 60,
	list : shirt.qtyValues,
	index : 0,
	caption : {
    y : 15,
    textNode : "Qty"
  },
  fn : function (evt) {
		this.owner.setQty(this.index);
	}
});
