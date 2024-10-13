
//=============================   Pergola examples - nut   ==========================
var nut = "m132,116c14,-17 13,-65 6,-85.9C132,7.32 103,2.85 80.1,8.26C57.3,2.35 30.1,6.72 21,30.4C12.1,50.9 14.8,100 28,116c26.6,32 42.6,11 52.1,28c9.5,-17 25.9,4 51.9,-28";

$C({
  element : "path",
  d : nut,
  appendTo : $C({
    element : "clipPath",
    id : "clip",
    appendTo : pergola.defs
  })
});

$C({
  element : "path",
  d : nut,
  filter : pergola.filter.noise({
    baseFrequency : ".12 .05",
    seed : 1000,
    tableValues : {R : ".9 .05", G : ".42 .02", B : ".25 .01"}
  }),
  "clip-path" : "url(#clip)",
  appendTo : pergola.user
});


