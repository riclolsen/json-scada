
//=============================   Pergola examples - shapes   ==========================



var g = $C({element : "g", transform : "translate(20 10)", appendTo : pergola.user});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "translate(0 64) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "translate(80 -.5) scale(4)",
    fill : "#FFFFC0",
    stroke : "black",
    "stroke-width" : .25
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "translate(220 -2) scale(4)",
    fill : "#FFFFC0",
    stroke : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "rotate(12 32 27.7128) translate(376 -72) scale(4)",
    fill : "#FFFFC0",
    stroke : "gray",
    "stroke-width" : 2,
    "stroke-linejoin" : "round"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "translate(376 -.5) scale(4) skewX(42)",
    fill : "#FFFFC0",
    stroke : "black",
    "stroke-width" : .25
  }
});


g = $C({element : "g", transform : "translate(20 220)", appendTo : pergola.user});

pergola.use({
  parent : g,
  shape : pergola.shapes.rhombus,
  attributes : {
    transform : "translate(0 0) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.pentagon,
  attributes : {
    transform : "translate(120 0) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.hexagon,
  attributes : {
    transform : "translate(240 0) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.heptagon,
  attributes : {
    transform : "translate(360 0) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.octagon,
  attributes : {
    transform : "translate(480 0) scale(2)",
    fill : "lightsteelblue"
  }
});


g = $C({element : "g", transform : "translate(20 340)", appendTo : pergola.user});

pergola.use({
  parent : g,
  shape : pergola.shapes.nonagon,
  attributes : {
    transform : "translate(0 0) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.decagon,
  attributes : {
    transform : "translate(120 0) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.dodecagon,
  attributes : {
    transform : "translate(240 0) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.semicircle,
  attributes : {
    transform : "translate(360 0) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.star,
  attributes : {
    transform : "translate(480 0) scale(1.2)",
    fill : "lightsteelblue"
  }
});











new pergola.Legend()
.build({
	x : 42,
	y : 460,
	vSpacing : 12,
	legend : {
		item1 : {
			caption : [
				'Preset shapes are definitions stored in the shapes library.',
			  'A shape object defines an SVG element and its geometrical',
			  'attributes only. No paint attributes or style.'
			]
		},
		item2 : {
			caption : [
				'You can use a shape by invoking the utility function pergola.use(),',
			  'which simulates the SVG <use> element. Because only the geometry',
			  'is defined for shapes, you have total control over stroke width/scaling.'
			]
		},
		item3 : {
			caption : [
				'Some classes are enabled to process the instance property "shape"',
			  '(use cases in the buttons example).'
			]
		}
	}
});
