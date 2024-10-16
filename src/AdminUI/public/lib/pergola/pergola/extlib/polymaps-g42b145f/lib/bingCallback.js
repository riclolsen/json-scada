
function bingCallback(data) {
	var
    win = pergola.Window.active(),
    resourceSets = data.resourceSets,
    resources,
    resource,
    i, j,
    a,
    lines,
    text;

	for (i = 0; i < resourceSets.length; i++) {
		resources = resourceSets[i].resources;
		for (j = 0; j < resources.length; j++) {
			resource = resources[j];
			win.map.add(polymaps.image(win.currentView)
				.url(template(resource.imageUrl, resource.imageUrlSubdomains)))
				.tileSize({x: resource.imageWidth, y: resource.imageHeight});
		}
	}

  win.logoMaker({symbol : data.brandLogoUri, x : 7, y : -18, width : "64", height : "64"});

	if (!win.copyright) {
    a = data.copyright.split(" ");
  	lines = [];
  	for (i = 4; i > 1; i--) lines.push(a.splice(0, parseInt(a.length / i + (2 - (i == 4)))).join(" "));
  	lines.push(a.splice(0).join(" "));
		win.copyright = $C({element: "g", transform: "translate(13 " + (win.doc.visibleH - 54) + ")", "pointer-events": "none", appendTo: win.doc.container});
		$C({element: "rect", x: 2.5, y: .5, rx: 4, width: 254, height: 50, fill: "black", opacity: .4, appendTo: win.copyright});
		text = $C({element: "text", "font-size": "6.5pt", fill: "#EFEFEF", appendTo: win.copyright});
		for (i = 0; i < lines.length; i++) $C({element: "tspan", x: 5, dy: "8.5pt", textNode: lines[i], appendTo: text});
	}
};


// Returns a Bing URL template given a string and a list of subdomains.
function template(url, subdomains) {
	var n = subdomains.length,
	salt = ~~(Math.random() * n); // per-session salt
// Returns the given coordinate formatted as a 'quadkey'.
	function quad(column, row, zoom) {
		var key = "";
		for (var i = 1; i <= zoom; i++) {
			key += (((row >> zoom - i) & 1) << 1) | ((column >> zoom - i) & 1);
		}
		return key;
	}
	return function(c) {
		var quadKey = quad(c.column, c.row, c.zoom),
		server = Math.abs(salt + c.column + c.row + c.zoom) % n;
		return url
		.replace("{quadkey}", quadKey)
		.replace("{subdomain}", subdomains[server]);
	};
}


function queryBing(view) {
	this.tilesQueryScript = $C({
		element : "script",
		type : "text/javascript", 
		"xlink:href" : "http://dev.virtualearth.net"
			+ "/REST/V1/Imagery/Metadata/"
      + (view || "aerialWithLabels")
			+ "?key=AoQhfBJhkyflrvYyCnfMkNWqHws6AAgZKYXPLK4ME1oRQqdygndkTDDrLjAQoM32"
			+ "&jsonp=bingCallback",
		appendTo: pergola.defs
	});
}

