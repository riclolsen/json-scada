
//=======================   Pergola examples - D3 multiple documents - force.js   =====================


var forceWin = new pergola.Window("D3 Force-Directed Graph");

/* DS-
 * Cleanest way to encapsulate in window:
 * Define a helper function as instance method for the "contains" property
 * of the window object. The D3 example script is the body of the function.
*/
forceWin.contents = function () {
  var w = 960,
	    h = 500,
      fill = d3.scale.category20(),
      o = this;

// DS- replace <svg> with <g> (BBox needed).
  this.vis = d3.select(this.doc.transformable)
    .append("svg:g");

// DS- sets a stable BBox
	$C({element: "rect", width: w, height: h, fill: "none", "pointer-events": "none", appendTo: this.vis.node()});

  d3.json("miserables.json", function(json) {
    o.force = d3.layout.force()
        .charge(-120)
        .linkDistance(30)
        .nodes(json.nodes)
        .links(json.links)
        .size([w, h])
        .start();
  
    o.link = o.vis.selectAll("line.link")
        .data(json.links)
      .enter().append("svg:line")
        .attr("class", "link")
        .attr("opacity", function(d) { return Math.sqrt(d.value) / 10; })
        .attr("x1", function(d) { return d.source.x; })
        .attr("y1", function(d) { return d.source.y; })
        .attr("x2", function(d) { return d.target.x; })
        .attr("y2", function(d) { return d.target.y; });
  
    o.node = o.vis.selectAll("circle.node")
        .data(json.nodes)
      .enter().append("svg:circle")
        .attr("class", "node")
        .attr("cx", function(d) { return d.x; })
        .attr("cy", function(d) { return d.y; })
        .attr("r", 5)
        .attr("fill", function(d) { return fill(d.group); })
        .call(o.force.drag);
  
    o.node.append("svg:title")
        .text(function(d) { return d.name; });
  
    o.vis.transition()
        .duration(1000);

/*
 * DS- For some unidentified reason the line nodes trigger the event
 * systematically, but the circle nodes only erratically.
*/
//  this.vis.node().addEventListener("mousedown", xWindowinteractivity, false);

// Registering evnts on the circle nodes individually (see comment at line 27).
    for (var a in o.node[0]) o.node[0][a].addEventListener("mousedown", xWindowinteractivity, false);

    o.force.on("tick", function() {
      o.link.attr("x1", function(d) { return d.source.x; })
          .attr("y1", function(d) { return d.source.y; })
          .attr("x2", function(d) { return d.target.x; })
          .attr("y2", function(d) { return d.target.y; });
  
      o.node.attr("cx", function(d) { return d.x; })
          .attr("cy", function(d) { return d.y; });
    });
  });
};


function xWindowinteractivity(evt) {
  evt.stopPropagation;
  streamWin.transition(streamWin.transitTo, evt.target.getAttributeNS(null, "fill"));
}



forceWin.build({
  width: 570,
  height: 440,
  contains: function () {return this.contents();}
});
