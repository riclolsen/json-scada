
//=============================   Pergola examples - patterns   ==========================



var g = $C({element : "g", transform : "translate(20 20)", appendTo : pergola.user});

// A couple of gradients. Some other referenced gradients are defined in the skin.es file
var gradient = $C({element : "linearGradient", id : "rod", x1 : "0%", y1 : "0%", x2 : "100%", y2 : "0%", gradientUnits : "objectBoundingBox", appendTo : g});
$C({element : "stop", offset : "0%", "stop-color" : "#C0C0C0", appendTo : gradient});
$C({element : "stop", offset : "100%", "stop-color" : "#404040", appendTo : gradient});

gradient = $C({element : "linearGradient", id : "fade_redGradient", x1 : "0%", y1 : "0%", x2 : "100%", y2 : "0%", gradientUnits : "objectBoundingBox", appendTo : g});
$C({element : "stop", offset : "0%", "stop-color" : "#FF8080", appendTo : gradient});
$C({element : "stop", offset : "100%", "stop-color" : "#602020", appendTo : gradient});

gradient = $C({element : "linearGradient", id : "rooftile", x1 : "0%", y1 : "0%", x2 : "100%", y2 : "0%", gradientUnits : "objectBoundingBox", appendTo : g});
$C({element : "stop", offset : "0%", "stop-color" : "#EE7777", appendTo : gradient});
$C({element : "stop", offset : "8%", "stop-color" : "#F27979", appendTo : gradient});
$C({element : "stop", offset : "100%", "stop-color" : "#602020", appendTo : gradient});



// =======================  catseye  ============================

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "catseye()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.catseye({transform : "scale(1.5 1)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.catseye({colors : ["#900000", "#B00000"], transform : "scale(3)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.catseye({colors : ["#F8F8FA", "#EEEEF0"], transform : "scale(2) rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : pergola.pattern.catseye({colors : ["#F8F8FA", "#EEEEF0"], transform : "scale(4)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.catseye({colors : ["black", "gray"], transform : "scale(.8) rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.catseye({colors : ["#5E5E64", "#7E7E84"], transform : "scale(.8)"}),
  appendTo : g
});



// =======================  pirelli  ============================

g = $C({element : "g", transform : "translate(20 140)", appendTo : pergola.user});

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "pirelli()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.pirelli({transform : "scale(6)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.pirelli({colors : ["#4060B0", "#3050A0"], transform : "scale(2.5)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.pirelli({colors : ["darkslategray", "#909090"], transform : "scale(.5)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : pergola.pattern.pirelli({colors : ["url(#sliderPatternGrad)", "#608070"], transform : "scale(3.75)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.pirelli({colors : ["url(#sliderPatternGrad)", "#80A0C0"], opacity : .75, transform : "scale(4) rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.pirelli({colors : ["url(#sliderPatternGrad)", "gainsboro"], transform : "scale(2 6) rotate(45)"}),
  appendTo : g
});



// =======================  line  ============================

g = $C({element : "g", transform : "translate(20 260)", appendTo : pergola.user});

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "line()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.line({colors : ["#D8D8D8"]}),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.line({spacing : 9}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.line({colors : ["#A0A0A0"], size : 8, transform : "rotate(90)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : pergola.pattern.line({colors : ["#507060"], size : .5, spacing : 2, transform : "rotate(80)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.line({colors : ["wheat"], size : 1, spacing : 5, transform : "rotate(-45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.line({colors : ["#FFFFD0"], size : 17, spacing : 6}),
  appendTo : g
});



// =======================  grid  ============================

g = $C({element : "g", transform : "translate(20 380)", appendTo : pergola.user});

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "grid()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.grid(),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.grid({colors : ["#C0C0D8"], spacing : 12}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.grid({colors : ["pink"], spacing : 8, transform : "skewX(60)"}),
  appendTo : g
});


// =======================  bar  ============================

// Legend
$C({element : "text", x : 360, y : -6, fill : "#808080", textNode : "bar()", appendTo : g});

$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : "url(#fade_redGradient)",
  appendTo : g
});
$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : pergola.pattern.bar({colors : ["none", "#B05050"], size : 1, spacing : 2}),
  appendTo : g
});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.bar({colors : ["none", "url(#rooftile)"], size : 40, spacing : 0, transform : "rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.bar({colors : ["black", "url(#rod)"], size : 4, spacing : 12}),
  appendTo : g
});



// =======================  grille  ============================

g = $C({element : "g", transform : "translate(20 500)", appendTo : pergola.user});

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "grille()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.grille({colors : ["goldenrod", "white"], transform : "scale(2 6) rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.grille({colors : ["#707890", "black"], transform : "scale(8) rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.grille({colors : ["#80A0C0", "whitesmoke"], transform : "scale(4 3)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 360,
  rx : 4,
  width : 90,
  height : 90,
  fill : "none",
  stroke : pergola.pattern.grille({colors : ["gainsboro", "slategray"]}),
  "stroke-width" : 12,
  appendTo : g
});

// Legend
$C({element : "text", x : 480, y : -6, fill : "#808080", textNode : "pied_de_poule()", appendTo : g});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.pied_de_poule(),
  appendTo : g
});

// Legend
$C({element : "text", x : 600, y : -6, fill : "#808080", textNode : "pied_de_poule1()", appendTo : g});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.pied_de_poule1(),
  appendTo : g
});



// =======================  checkers  ============================

g = $C({element : "g", transform : "translate(20 620)", appendTo : pergola.user});

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "checkers()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.checkers(),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.checkers({colors : ["seagreen", "white"], transform : "scale(.25) skewX(20)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.checkers({colors : ["#80A0C0", "oldlace"], transform : "scale(1 2.75) rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : pergola.pattern.checkers({colors : ["lightslategray", "white"], transform : "scale(1 .125)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.checkers({colors : ["brown", "green"], transform : "rotate(90) scale(.5 .125)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.checkers({colors : ["rosybrown", "antiquewhite"], transform : "scale(1.5 .125) rotate(45)"}),
  appendTo : g
});



// =======================  dot  ============================

g = $C({element : "g", transform : "translate(20 740)", appendTo : pergola.user});

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "dot()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.dot(),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.dot({colors : ["#202020", "white"], spacing : 3, size : 1}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.dot({colors : ["#202020", "white"], spacing : 6, size : 2.2, transform : "rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : pergola.pattern.dot({colors : ["#D8C0B0", "#4060A0"], spacing : 8, size : 4, opacity : .65, transform : "scale(2.5 1)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.dot({colors : ["brown", "url(#paddleReflection)"], spacing : 24, size : 4}),
  appendTo : g
});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.dot({colors : ["yellowgreen", "gold"], spacing : 16, size : 8, transform : "rotate(45)"}),
  appendTo : g
});



// =======================  hive  ============================

g = $C({element : "g", transform : "translate(20 860)", appendTo : pergola.user});

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "hive()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.hive(),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.hive({colors : ["white", "#707890"], "stroke-width" : 3, transform : "scale(1 .25)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.hive({colors : ["darkred", "#786060"], "stroke-width" : 2, transform : "scale(1.5 .75) rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : pergola.pattern.hive({colors : ["whitesmoke", "lightslategray"], width : 32, "stroke-width" : 11, transform : "rotate(-90) scale(.5)", opacity : .6}),
  appendTo : g
});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.hive({colors : ["papayawhip", "cornflowerblue"], width : 8, "stroke-width" : 2}),
  appendTo : g
});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.hive({colors : ["darkseagreen", "aquamarine"], width : 16, "stroke-width" : 1, transform : "rotate(-90) scale(.5)"}),
  appendTo : g
});



// =======================  ring  ============================

g = $C({element : "g", transform : "translate(20 980)", appendTo : pergola.user});

// Legend
$C({element : "text", y : -6, fill : "#808080", textNode : "ring()", appendTo : g});

$C({
  element : "rect",
  width : 90,
  height : 90,
  fill : pergola.pattern.ring(),
  appendTo : g
});

$C({
  element : "rect",
  x : 120,
  width : 90,
  height : 90,
  fill : pergola.pattern.ring({colors : ["#A0A0A0", "url(#pushpinGrad_blue)", "url(#pushpinGrad_red)"], spacing : 17, size : 8, "stroke-width" : 6, transform : "rotate(45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 240,
  width : 90,
  height : 90,
  fill : pergola.pattern.ring({colors : ["none", "url(#pushpinGrad_blue)", "gold"], spacing : 16, size : 24, "stroke-width" : 4}),
  appendTo : g
});

$C({
  element : "rect",
  x : 360,
  width : 90,
  height : 90,
  fill : pergola.pattern.ring({colors : ["none", "navy", "darkred"], spacing : 16, size : 20, "stroke-width" : 2}),
  appendTo : g
});

$C({
  element : "rect",
  x : 480,
  width : 90,
  height : 90,
  fill : pergola.pattern.ring({colors : ["#000000", "yellowgreen", "gold"], spacing : 14, size : 14, "stroke-width" : 11, opacity : .5, transform : "rotate(-45)"}),
  appendTo : g
});

$C({
  element : "rect",
  x : 600,
  width : 90,
  height : 90,
  fill : pergola.pattern.ring({colors : ["none", "yellowgreen", "gold"], spacing : 14, size : 12, "stroke-width" : 4}),
  appendTo : g
});






