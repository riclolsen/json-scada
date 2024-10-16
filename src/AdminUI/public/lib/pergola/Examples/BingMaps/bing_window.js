
//====================   Pergola - Bing tiles window example  ==========================


/*
 *                                ATTENTION !
 *
 * THIS WORK USES A MODIFIED, UNOFFICIAL AND UNPUBLISHED VERSION OF POLYMAPS.
 * IT IS FORBIDDEN TO DISTRIBUTE THIS VERSION OF POLYMAPS.
 *
 * Latest Polymaps release at: http://polymaps.org/
*/


var bingWin = new pergola.Window("Bing Maps")
.build({
  isFull : true,
  type : "map",
  canvas : {
		width : 2048,               // multiple of 256. Should be >= window.screen.width
		height : 1536              // multiple of 256. Should be >= window.screen.height
	},
  fill : "#010413",
  view : "aerialWithLabels",
  menu : {
    views : {
      title : "Views",
      items : {
        aerial : {
          string : "Aerial",
          check : false,
          exclusive : true,
          view : "aerial",
          fn : tileSource
        },
        aerialLabels : {
          string : "Aerial With Labels",
          check : true,
          exclusive : true,
          view : "aerialWithLabels",
          fn : tileSource
        },
        road : {
          string : "Road",
          check : false,
          exclusive : true,
          view : "road",
          fn : tileSource
        }
      }
    },
    layers : {
      title : "Layers",
      items : {
        lukanga : {
          string : "Lukanga Swamp Rally",
          check : false,
          target : function () {
            return {
              layer : pergola.Window.active().layers.lukangaRally,
              center : {lat : -14.46, lon : 27.3125},
              zoom : 11,
              view : "aerialWithLabels"
            }
          },
          fn : 'toggleLayer'
        },
        polygons : {
          string : "Polygons",
          check : false,
          target : function () {
            return {
              layer : pergola.Window.active().layers.polygons,
              center : {lat : 37.7590, lon : -122.4191},
              zoom : 14,
              view : "road"
            }
          },
          fn : 'toggleLayer'
        },
        bananas : {
          string : "Top 10 banana producing nations",
          check : false,
          target : function () {
            return {
              layer : pergola.Window.active().layers.bananas,
              center : {lat : 10, lon : 100},
              zoom : 4,
              view : "aerial"
            }
          },
          fn : 'toggleLayer',
          separator : new pergola.Separator()
        },
        copyright : {
          string : "Copyright",
          check : true,
          target : function () {return pergola.Window.active().doc.copyright;},
          fn : function () {
            if (!this.target()) return;
            var l = pergola.Window.active().layers.copyright;
            l.display = l.display == "block" ? "none" : "block";
            this.target().setAttributeNS(null, "display", l.display);
          }
        }
      }
    },
    go_places : {
      title : "Go Places",
      items : {
        paris : {
          string : "Paris",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : 48.8553, lon : 2.3456}); c.mapZoom(16);}
        },
        rome : {
          string : "Rome",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : 41.9030, lon : 12.4664}); c.mapZoom(14);}
        }
        ,
        tokyo : {
          string : "Tokyo",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : 35.6429, lon : 139.8098}); c.mapZoom(11);}
        },
        newyork : {
          string : "New York",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : 40.7050, lon : -74.0093}); c.mapZoom(11);}
        },
        sydney : {
          string : "Sydney",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : -33.8654, lon : 151.2102}); c.mapZoom(12);}
        },
        venice : {
          string : "Venice",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : 45.4351, lon : 12.3375}); c.mapZoom(14);}
        },
        riodejaneiro : {
          string : "Rio De Janeiro",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : -22.9389, lon : 316.7979}); c.mapZoom(12);}
        },
        buenosaires : {
          string : "Buenos Aires",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : -34.6570, lon : 301.6016}); c.mapZoom(11);},
          separator : new pergola.Separator()
        },
        northAmerica : {
          string : "North America",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : 42.37, lon : 268.07}); c.mapZoom(3);}
        },
        india : {
          string : "India",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : 18.832, lon : 78.734}); c.mapZoom(5);}
        },
        wEurope : {
          string : "Western Europe",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : 46.588, lon : 5.938}); c.mapZoom(5);}
        },
        patagonia : {
          string : "Patagonia",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : -50.570, lon : -70.977}); c.mapZoom(7);}
        },
        antartica : {
          string : "Antartica",
          fn : function () {var c = pergola.Window.active(); c.centerMap({lat : -79.1, lon : -10.5}); c.mapZoom(2);},
          separator : new pergola.Separator()
        },
        svgOpen2011 : {
          string : "SVG Open 2011",
          target : function () {
            return {
              layer : pergola.Window.active().layers.svgOpen2011,
              center : {lat : 42.36131, lon : -71.08124},
              zoom : 17,
              view : "road"
            };
          },
          fn : 'toggleLayer'
        }
      }
    },
    zoomLevel : {
      title : "Levels",
      items : {
        z1 : {string : "1", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(1);}},
        z2 : {string : "2", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(2);}},
        z3 : {string : "3", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(3);}},
        z4 : {string : "4", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(4);}},
        z5 : {string : "5", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(5);}},
        z6 : {string : "6", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(6);}},
        z7 : {string : "7", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(7);}},
        z8 : {string : "8", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(8);}},
        z9 : {string : "9", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(9);}},
        z10 : {string : "10", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(10);}},
        z11 : {string : "11", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(11);}},
        z12 : {string : "12", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(12);}},
        z13 : {string : "13", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(13);}},
        z14 : {string : "14", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(14);}},
        z15 : {string : "15", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(15);}},
        z16 : {string : "16", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(16);}},
        z17 : {string : "17", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(17);}},
        z18 : {string : "18", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(18);}},
        z19 : {string : "19", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(19);}},
        z20 : {string : "20", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(20);}},
        z21 : {string : "21", check : false, exclusive : true, fn : function () {pergola.Window.active().mapZoom(21);}}
      }
    },
    grid : {
      title : "Grid",
      items : {
        graticule : {
          string : "Graticule",
          check : false,
          fn : function () {
            var win = pergola.Window.active(),
								map = win.map,
                l = win.layers.graticule;
            l.display = l.display == "block" ? "none" : "block";
            if (!map.grid) {
              map.add(polymaps.grid());
              map.center(map.center());
            }
            map.grid.setAttributeNS(null, "display", l.display);
          }
        }
      }
    },
    unit : {
      title : "Unit",
      items : {
        km : {
          string : "Kilometres",
          check : true,
          exclusive : true,
          fn : function () {
          	var win = pergola.Window.active();
						win.map.unit = "Km";
						if (win.doc.itinerary) win.doc.itinerary.updateUnit(win.map);
					}
        },
        mi : {
          string : "Miles",
          check : false,
          exclusive : true,
          fn : function () {
          	var win = pergola.Window.active();
						win.map.unit = "mi";
						if (win.doc.itinerary) win.doc.itinerary.updateUnit(win.map);
					}
        },
        nmi : {
          string : "Nautical Miles",
          check : false,
          exclusive : true,
          fn : function () {
						var win = pergola.Window.active();
						win.map.unit = "nmi";
						if (win.doc.itinerary) win.doc.itinerary.updateUnit(win.map);
					}
        }
      }
    }
  },
  views : {
    aerial : {},
    aerialWithLabels : {},
    road : {}
  },
  layers : {
    copyright : {
      feature : false,
      display : "block"
    },
    bananas : {
      feature : true,
      display : "none"
    },
    polygons : {
      feature : true,
      display : "none"
    },
    lukangaRally : {
      feature : true,
      display : "none"
    },
    svgOpen2011 : {
      feature : true,
      display : "none"
    },
    graticule : {
      feature : false,
      display : "none"
    }
  },
  release : function () {
    polymaps.origin.x = this.x + this.doc.x;
    polymaps.origin.y = this.y + this.doc.y;
    this.centerMap(this.map.center());
    if (this.copyright) this.copyright.setAttributeNS(null, "transform", "translate(13 " + (this.doc.visibleH - 54) + ")");
  },
  contains : function () {return this.mapMaker()}
});


/*
 * MENU USER FUNCTIONS
 * These functions are instance methods (or invoked as). See Pergola documentation
 * for possible values of the "fn" property (user function).
*/


function tileSource() {
	var
    win = pergola.Window.active(),
		container = win.map.container();

	if (win.currentView == win.views[this.view]) return;

	container.removeChild(container.firstChild);
	win.currentView = win.views[this.view];
	pergola.defs.removeChild(win.tilesQueryScript);
  queryBing.call(win, this.view);
}


var m = bingWin.menu.layers.list;

m.polygons.toggleLayer = function (evt) {
  var target = this.target(),
      o = target.layer,
      win = pergola.Window.active();

  if (target.view) {
    win.mapViewsToggle(target.view);
  }

  if (!o.container) {
    win.map
    .add(polymaps.geoJson(o)
    .features([
      {
        "geometry" : {
          "coordinates" : [[[
            [-122.43136, 37.76932],
            [-122.42019, 37.77002],
            [-122.40788, 37.76922],
            [-122.40603, 37.75053],
            [-122.40530, 37.74923],
            [-122.40780, 37.74841],
            [-122.4225, 37.7481],
            [-122.42927, 37.74759],
            [-122.43136, 37.76932]
          ]]],
          "type" : "MultiPolygon",
          "style" : "fill: #C0C4C8; fill-opacity: .3; stroke: #0080FF; stroke-width: 2px;"
        }
      },
      {
        "geometry" : {
          "type" : "Point",
          "coordinates" : [-122.43136, 37.76932],
          "elements" : [{element : "circle", r : 5, fill : "red", "fill-opacity" : .4, stroke : "red"}]
        }
      }
    ]));
  }
  win.centerMap(target.center);
  win.mapZoom(target.zoom);
  win.showMapFeatureLayer(o);
}


m.lukanga.toggleLayer = function (evt) {
  var target = this.target(),
      o = target.layer,
      win = pergola.Window.active();

  if (target.view) {
    win.mapViewsToggle(target.view);
  }

  if (!o.container) {
    var leg = pergola.symbols.leg;
    win.map.add(polymaps.geoJson(o)
    .features([
      {
        "geometry" : {
          "type" : "Point",
          "coordinates" : [27.0498, -14.3668],
          "elements" : leg
        }
      },
      {
        "geometry" : {
          "type" : "Point",
          "coordinates" : [27.2329, -14.4837],
          "elements" : leg
        }
      },
      {
        "geometry" : {
          "type" : "Point",
          "coordinates" : [27.541, -14.434],
          "elements" : leg
        }
      },
      {
        "geometry" : {
          "coordinates" : [
            [27.0498, -14.3668], [27.0488, -14.38], [27.0535, -14.3812], [27.0835, -14.3745], [27.1056, -14.3792], [27.1296, -14.3702], [27.132, -14.3702], [27.142, -14.3752], [27.152, -14.3865], [27.1678, -14.3871], [27.1812, -14.3708], [27.193, -14.3708], [27.1998, -14.3837], [27.2265, -14.396], [27.2265, -14.420], [27.2072, -14.4428], [27.2065, -14.4628], [27.1955, -14.4845], [27.1956, -14.4899], [27.203, -14.4922], [27.2329, -14.4837], [27.2601, -14.4837], [27.2678, -14.4666], [27.2868, -14.4725], [27.3, -14.4666], [27.3091, -14.4766], [27.3165, -14.4766], [27.3292, -14.4512], [27.3525, -14.4350], [27.3414, -14.4053], [27.3442, -14.4015], [27.363, -14.4075], [27.3717, -14.4048], [27.38, -14.4076], [27.4063, -14.4017], [27.4379, -14.4], [27.4629, -14.3907], [27.4722, -14.3899], [27.4835, -14.379], [27.5198, -14.3766], [27.522, -14.3799], [27.5152, -14.3928], [27.5231, -14.4069], [27.52, -14.4168], [27.5002, -14.42], [27.4987, -14.4253], [27.5137, -14.434], [27.5221, -14.4324], [27.5326, -14.4368], [27.541, -14.434]
          ],
          "type" : "LineString",
          "style" : "fill: none; stroke: #FF00F0; stroke-width: 2px;"
        }
      }
    ]));
  }
  win.centerMap(target.center);
  win.mapZoom(target.zoom);
  win.showMapFeatureLayer(o);
}


m.bananas.toggleLayer = function (evt) {
  var target = this.target(),
      o = target.layer,
      win = pergola.Window.active();

  if (target.view) {
    win.mapViewsToggle(target.view);
  }

  if (!o.container) {
    var banana = pergola.symbols.banana,
        node,
        features = [],
        prod = [
          {coordinates : [79, 18], tag : "INDIA (1) 26.2 M t"},
          {coordinates : [122, 13.62], tag : "PHILIPPINES (2) 9 M t", scale : "(.94)"},
          {coordinates : [112, 25], tag : "CHINA (3) 8.2 M t", scale : "(.88)"},
          {coordinates : [-80.1116, 0], tag : "ECUADOR (4) 7.6 M t", scale : "(.82)"},
          {coordinates : [-49, -2], tag : "BRAZIL (5) 7.2 M t", scale : "(.76)"},
          {coordinates : [110, 0], tag : "INDONESIA (6) 6.3 M t", scale : "(.7)"},
          {coordinates : [-97.1771, 18.5304], tag : "MEXICO (7) 2.2 M t", scale : "(.64)"},
          {coordinates : [-84.1583, 9.4857], tag : "COSTA RICA (8) 2.1 M t", scale : "(.58)"},
          {coordinates : [-77.1051, 5.7622], tag : "COLOMBIA (9) 2 M t", scale : "(.52)"},
          {coordinates : [100, 16.3], tag : "THAILAND (10) 1.5 M t", scale : "(.46)"}
        ];
/*
 * Using Pergola's String prototype extension method width() to compute the text
 * width and assign the result to the property "width" for each object in "prod".
*/
    for (var a in prod) prod[a].width = prod[a].tag.width("10px");

    function tag(i) {
      return [
        {element : "rect", x : .5, y : -15.5, width : parseInt(prod[i].width) + 8, height : 12, fill : "url(#quickTipGrad)", stroke : "#808080"},
        {element : "text", x : 4, y : -6.5, "font-size" : "10px", "pointer-events" : "none", textNode : prod[i].tag}
      ];
    };

    function geometry(i, obj) {
      var scale = (obj == banana) ? prod[i].scale : 0;
      return {
        "geometry" : {
          "type" : "Point",
          "coordinates" : prod[i].coordinates,
          "elements" : obj,
          "scale" : scale,
        }
      };
    };

    for (var i in prod) {
      features.push(geometry(i, banana));
      features.push(geometry(i, tag(i)));
    }
    win.map.add(polymaps.geoJson(o).features(features));
  }

  win.centerMap(target.center);
  win.mapZoom(target.zoom);
  win.showMapFeatureLayer(o);
}


bingWin.menu.go_places.list.svgOpen2011.toggleLayer = function (evt) {
  var target = this.target(),
      o = target.layer,
      win = pergola.Window.active();

  if (target.view) {
    win.mapViewsToggle(target.view);
  }

  if (!o.container) {
    win.map.add(polymaps.geoJson(o)
    .features([
      {
        "geometry" : {
          "type" : "Point",
          "coordinates" : [-71.08124, 42.36131],
          "elements" : pergola.symbols.signalPaddle
        }
      }
    ]));
  }
  win.centerMap(target.center);
  win.mapZoom(target.zoom);
  win.showMapFeatureLayer(o);
}
