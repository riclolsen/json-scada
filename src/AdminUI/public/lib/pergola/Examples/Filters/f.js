
//=============================   Pergola examples - Filters   ==========================


var filters = [
  {name : "noise() – Rosewood", uri : pergola.filter.noise({baseFrequency : ".1 0", seed : 2})},
  {name : "turbulence() – Burr-elm", uri : pergola.filter.turbulence({baseFrequency : ".05 .055", seed : 1000, tableValues : {R : ".31 0", G : ".1 0", B : ".03 0"}})},
  {name : "noise() – Walnut burl", uri : pergola.filter.noise({baseFrequency : ".055 .035", seed : 1000, tableValues : {R : ".21 0", G : ".08 0", B : ".025 0"}})},
  {name : "noise() – Spruce", uri : pergola.filter.noise({baseFrequency : ".7 0", numOctaves : 1, values : "1.5 1 1 1 1 1.5 0 0 0 0 1.5 0 0 0 0 0 0 0 0 1.5", tableValues : {R : ".15 .7", G : ".15 .5", B : ".15 .15"}})},
  {name : "noise() – Pine", uri : pergola.filter.noise({baseFrequency : ".03 0", values : "1 1 1 0 0 1 0 0 0 0 1 0 0 0 0 1 0 0 0 1", tableValues : {R : ".3 1.1", G : ".3 1.15", B : "0 .65"}})},
  {name : "noise() – Hay stack", uri : pergola.filter.noise({baseFrequency : "1 .05", tableValues : {R : ".6 0", G : ".6 0", B : ".05 0"}})},
  {name : "noise() – Savana", uri : pergola.filter.noise({baseFrequency : ".2 .005", tableValues : {R : ".2 0", G : ".4 0", B : ".05 0"}})},
  {name : "noise() – Red velvet curtain", uri : pergola.filter.noise({baseFrequency : ".03 .002", tableValues : {R : ".16 0", G : "0 0", B : "0 0"}})},
//  {name : "turbulence() – Stone", uri : pergola.filter.turbulence({baseFrequency : ".9999 .9999", tableValues : {R : ".4 .9", G : ".4 .9", B : ".475 .99"}})},
  {name : "noise() – Stone", uri : pergola.filter.noise({baseFrequency : ".84 .85", tableValues : {R : ".45 .08", G : ".5 .1", B : ".5 .1"}})},
  {name : "turbulence() – Stone", uri : pergola.filter.turbulence({baseFrequency : ".056 .011", tableValues : {R : ".82 .7", G : ".82 .7", B : ".82 .7"}})},
  {name : "specular() – Bevel (default)", uri : pergola.filter.specular()},
  {name : "specular() – Inset", uri : pergola.filter.specular({surfaceScale : 4, specularExponent : 2, specularConstant : 1.1, "fePointLight" : {x : 10000, y : 10000, z : -1000}})},
  {name : "blur() – Drop shadow (default)", uri : pergola.filter.blur()},
  {name : "blur() – Drop shadow", uri : pergola.filter.blur({stdDeviation : 3, dx : 4.5, dy : 4.5, "flood-color" : "#103058", "flood-opacity" : .6})},
  {name : "blur() – Drop shadow", uri : pergola.filter.blur({stdDeviation : 2, dx : 2, dy : 2, "flood-opacity" : 1})}
];

var g = $C({element : "g", transform : "translate(20 20)", "font-family" : "'Segoe UI', 'Trebuchet MS', 'Lucida Grande', 'Deja Vu'", "font-size" : 11, appendTo : pergola.doc});

for (var i in filters) {
	var x = i % 5 * 212 - .5,
			y = parseInt(i / 5) * 232 - .5;

  $C({
    element : "rect",
    x : x,
    y : y,
    width : 192,
    height : 192,
    fill : "white",
    stroke : "black",
    filter : filters[i].uri,
    appendTo : g
  });
  $C({
    element : "text",
    x : x,
		y : y + 208,
    textNode : filters[i].name,
    appendTo : g
  });
}

