
//=======================   Pergola examples - D3 multiple documents - worm.js   =====================

// DS: based on http://bl.ocks.org/1216850 by Jon Frost.

var wormWin = new pergola.Window("D3 Worm");

wormWin.contents = function() {
  var repCountTunnel = 200,
      repCountSpace = 100,
      mouse = [400, 400],
      zoom = 1,
      color = d3.scale.linear()
        .domain([0, repCountSpace])
        .interpolate(d3.interpolateHsl)
        .range(["hsl(250,100%,50%)", "hsl(180,100%,50%)"]),
      vis = d3.select(this.doc.transformable),
      node = vis.node();

  var gradient = $C({element : "linearGradient", id : "worm-gradient", x1 : "0%", y1 : "20%", x2 : "20%", y2 : "100%", appendTo : node});
  $C({element : "stop", offset : "20%", "stop-color" : "green", appendTo : gradient});
  $C({element : "stop", offset : "50%", "stop-color" : "blue", appendTo : gradient});
  $C({element : "stop", offset : "100%", "stop-color" : "orange", appendTo : gradient});

// Code for static centroid created used principles of the SVG-Replicate project.
  var tunnel = d3.select($C({element : "g", transform : "translate(150 54)", fill : "none", "stroke-width" : 4, "stroke-opacity" : .1, appendTo : node}))
    .selectAll()
      .data(d3.range(0, repCountTunnel, 1))
    .enter().append("svg:circle")
      .attr("r", function(d) { return d * .62 + 4})
      .attr("stroke", function(d) { return color(d); })
      .attr("transform", function(d) {
        return "rotate(" + d / 4 + ") translate(" + (d * 1.45).trim(3) + "," + (d * -.4).trim(3) + ")";
      });

  var g = d3.select(
    $C({element : "g", "stroke-width" : 5, "stroke-opacity" : .25, fill : "url(#worm-gradient)", appendTo : node})
  );

  var e = g.selectAll()
      .data(d3.range(repCountSpace))
    .enter().append("svg:ellipse")
      .attr("rx", function(d) { return (repCountSpace - d) * .8; })
      .attr("ry", function(d) { return (repCountSpace - d) * .5; })
      .attr("stroke", function(d) { return color(d); })
      .map(function(d) { return {center: [250, 250], angle: 30}; });

  g.timer = pergola.Timer()
  .initialize({
    handle : this,
    callback : function (timer) {
      timer.count ++;
      timer.target.attr("transform", function(d, i) {
        d.center[0] += ((mouse[0] / zoom - d.center[0]) / (i + 10));
        d.center[1] += ((mouse[1] / zoom - d.center[1]) / (i + 10));
        d.angle += Math.sin((timer.count + i) / 10) * 3;
        return "translate(" + d.center + ") rotate(" + d.angle + ")";
      });
    },
    frequence : 25,
    target : e,
    count : 0
  });

  this.registerEvents(this.doc.container, "mouseover", function (evt) {
    var doc = wormWin.doc,
        offset = {
//					x : doc.absoluteX(doc.container),
//	        y : doc.absoluteY(doc.container)
/*
 * workaround for Firefox getCTM() bug (https://bugzilla.mozilla.org/show_bug.cgi?id=873106)
*/
	        x : doc.owner.x + doc.x,
	        y : doc.owner.y + doc.y
				};

    zoom = doc.scaleFactor;
    pergola.dragarea.resize(offset.x, offset.y, doc.visibleW, doc.visibleH)
		.activate({
      handle : wormWin,
      callback : function (evt) {
        var m = pergola.mousePoint(evt);
        mouse[0] = m.x - this.offset.x;
        mouse[1] = m.y - this.offset.y;
      },
      offset : offset,
      updateCoordinates : false
    });
  });
  
}

wormWin.build({
  x : 120,
  y : 120,
  width : 600,
  height : 420,
  fill : "black",
  minimized: true,
  contains : function () {this.contents();}
});
