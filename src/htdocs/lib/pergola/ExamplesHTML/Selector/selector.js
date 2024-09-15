
//=======================   Pergola examples - popup list selector   =====================


// A commercial item
var tshirt = {container : $C({element : "g", transform : "translate(420 100)", appendTo : pergola.user})};

pergola.extend(tshirt, {
  x : 18.5,
  y : 14,
  shape : $C({element : "path", d : "M0 0h14q4.5 4.5,9 0h14v9h-5v24h-27v-24h-5z", transform : "scale(2)", fill : "#0040C0", appendTo : tshirt.container}),
  text : $C({element : "text", x : 37, y : 28, "font-size" : "10px", "font-weight" : "bold", "text-anchor" : "middle", kerning : 0, fill : "white", textNode : "PERGOLA", appendTo : tshirt.container}),
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



tshirt.sizeSelector = new pergola.Selector()
.build({
  owner : tshirt,
  parent : tshirt.container,
  x : -270,
  list : tshirt.sizes,
  index : 1,
  caption : {
    y : 15,
    textNode : "Size"
  },
  fn : function (evt) {
    this.owner.setSize(this.index);
  }
});

tshirt.colorSelector = new pergola.Selector()
.build({
  owner : tshirt,
  parent : tshirt.container,
  x : -270,
  y : 50,
  list : tshirt.colorNames,
  index : 2,
  caption : {
    y : 15,
    textNode : "Color"
  },
  fn : function (evt) {
    this.owner.setColor(this.index);
  }
});

tshirt.qtySelector = new pergola.Selector()
.build({
  owner : tshirt,
  parent : tshirt.container,
  x : -270,
  y : 100,
  width : 60,
  list : tshirt.qtyValues,
  index : 0,
  caption : {
    y : 15,
    textNode : "Qty"
  },
  fn : function (evt) {
    this.owner.setQty(this.index);
  }
});
