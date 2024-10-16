
//=============================   Pergola examples - shapes   ==========================



var g = $C({element : "g", transform : "translate(20 10)", appendTo : pergola.user});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "translate(0 32) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "translate(80 -.5) scale(3)",
    fill : "#FFFFC0",
    stroke : "black",
    "stroke-width" : .25
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "translate(194 -2) scale(3)",
    fill : "#FFFFC0",
    stroke : "black"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.triangle,
  attributes : {
    transform : "rotate(12 32 27.7128) translate(316 -62) scale(3)",
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
    transform : "translate(320 -.5) scale(3) skewX(42)",
    fill : "#FFFFC0",
    stroke : "black",
    "stroke-width" : .25
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.rhombus,
  attributes : {
    transform : "translate(0 180) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.pentagon,
  attributes : {
    transform : "translate(110 180) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.hexagon,
  attributes : {
    transform : "translate(220 180) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.heptagon,
  attributes : {
    transform : "translate(330 180) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.octagon,
  attributes : {
    transform : "translate(440 180) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.nonagon,
  attributes : {
    transform : "translate(0 310) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.decagon,
  attributes : {
    transform : "translate(110 310) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.dodecagon,
  attributes : {
    transform : "translate(220 310) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.semicircle,
  attributes : {
    transform : "translate(330 310) scale(2)",
    fill : "lightsteelblue"
  }
});

pergola.use({
  parent : g,
  shape : pergola.shapes.star,
  attributes : {
    transform : "translate(440 310) scale(1.2)",
    fill : "lightsteelblue"
  }
});


