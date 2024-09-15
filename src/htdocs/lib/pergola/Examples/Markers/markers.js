
//=============================   Pergola examples - markers   ==========================


var marker;

var g = $C({element: "g", appendTo: pergola.user});

$C({
  element : "path",
  d : "M100,100h100l50,50",
  fill : "none",
  stroke : "black",
  "stroke-width" : 4,
  "marker-end" : pergola.marker.arrow1(),
  appendTo : g
});


marker = pergola.marker.dot({radius : 13, fill : "#FFFF00", stroke : "blue"});
$C({
  element : "path",
  d : "M150.5,50.5v199h100v20h-190",
  fill : "none",
  stroke : "red",
  "marker-start" : marker,
  "marker-end" : marker,
  appendTo : g
});


marker = pergola.marker.dot();
$C({
  element : "path",
  d : "M100,200h100l50,50h50",
  fill : "none",
  stroke : "blue",
  "stroke-width" : 2,
  "marker-start" : marker,
  "marker-mid" : marker,
  "marker-end" : marker,
  appendTo : g
});


marker = pergola.marker.chevron();
$C({
  element : "path",
  d : "M130,325h50",
  fill : "none",
  stroke : "#808090",
  "stroke-width" : 8,
  "stroke-linecap" : "round",
  "marker-end" : marker,
  appendTo : g
});


$C({
  element : "path",
  d : "M230,325h50",
  fill : "none",
  stroke : "#808090",
  "stroke-width" : 4,
  "stroke-linecap" : "round",
  "marker-end" : marker,
  appendTo : g
});


$C({
  element : "path",
  d : "M310,325.5h40",
  fill : "none",
  stroke : "#808090",
  "stroke-width" : 1.2,
  "stroke-linecap" : "round",
  "marker-end" : marker,
  appendTo : g
});


marker = pergola.marker.chevronDbl();
$C({
  element : "path",
  d : "M140,375h40",
  fill : "none",
  stroke : "#808090",
  "stroke-width" : 8,
  "stroke-linecap" : "round",
  "marker-end" : marker,
  appendTo : g
});


$C({
  element : "path",
  d : "M240,375h40",
  fill : "none",
  stroke : "#808090",
  "stroke-width" : 4,
  "stroke-linecap" : "round",
  "marker-end" : marker,
  appendTo : g
});


$C({
  element : "path",
  d : "M320,375.5h40",
  fill : "none",
  stroke : "#808090",
  "stroke-width" : 1.2,
  "stroke-linecap" : "round",
  "marker-end" : marker,
  appendTo : g
});


$C({
  element : "path",
  d : "M140,425h36h36",
  fill : "none",
  stroke : "#808090",
  "stroke-width" : 4,
  "stroke-linecap" : "round",
  "marker-mid" : pergola.marker.secant(),
  appendTo : g
});


$C({
  element : "path",
  d : "M252,425h36h36",
  fill : "none",
  stroke : "#808090",
  "stroke-width" : 4,
  "stroke-linecap" : "round",
  "marker-mid" : pergola.marker.verticalBarDbl(),
  appendTo : g
});


$C({
  element : "path",
  d : "M80,550v-150",
  fill : "none",
  stroke : "darkorange",
  "stroke-width" : 3,
  "marker-start" : pergola.marker.terminalStart(),
  "marker-end" : pergola.marker.terminalEnd(),
  appendTo : g
});


marker = pergola.marker.intersect();
var ig = $C({element : "g", transform : "translate(180 472)", appendTo : g});
$C({
  element : "circle",
  cx : 64,
  cy : 55.424,
  r : 64,
  fill : "none",
  stroke : "gray",
  "stroke-width" : .25,
  appendTo : ig
});
$C({
  element : "polygon",
  points : "0,55.424 32,0 96,0 128,55.424 96,110.848 32,110.848",
  fill : "#e8ECdc",
  stroke : "black",
  "stroke-width" : .05,
  "marker-start" : marker,
  "marker-mid" : marker,
  "marker-end" : "none",
  appendTo : ig
});


$C({
  element : "path",
  d : "M370.5,425v100",
  fill : "none",
  stroke : "#D0D0D0",
  "marker-start" : pergola.marker.cross(),
  appendTo : g
});


$C({
  element : "path",
  d : "M340.5,50v50",
  fill : "none",
  stroke : "#B03030",
  "stroke-width" : 5,
  "marker-end" : pergola.marker.arrowBaseArc(),
  appendTo : g
});


$C({
  element : "path",
  d : "M370,75v50",
  fill : "none",
  stroke : "#208020",
  "stroke-width" : 6,
  "marker-end" : pergola.marker.arrowBaseArrow(),
  appendTo : g
});


$C({
  element : "path",
  d : "M440,75v.01",
  fill : "none",
  stroke : "red",
  "stroke-linecap" : "round",
  "stroke-width" : 6,
  "marker-end" : pergola.marker.bullseye(),
  appendTo : g
});


$C({
  element : "path",
  d : "M360,230h50",
  fill : "none",
  stroke : "#F8FF80",
  "stroke-width" : 12,
  "marker-end" : pergola.marker.marker(),
  appendTo : g
});









