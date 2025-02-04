
//=======================   Pergola examples - D3 multiple documents   =====================


var areaWin = new pergola.Window("D3 Area");


/* DS-
 * Cleanest way to encapsulate in window:
 * Define a helper function as instance method for the "contains" property
 * of the window object. The D3 example script is the body of the function.
*/

areaWin.contents = function() {
	var data = d3.range(20).map(function(i) {
	  return {x: i / 19, y: (Math.sin(i / 3) + 1) / 2};
	});
	
	var w = 450,
	    h = 275,
	    transX = 32,
	    transY = 20,
	    x = d3.scale.linear().domain([0, 1]).range([0, w]),
	    y = d3.scale.linear().domain([0, 1]).range([h, 0]);
	
	var vis = d3.select(areaWin.doc.transformable)
 
// DS- skip the <svg>. Not needed in this scenario.
//	  .append("svg:svg")
	  .data([data])
//	    .attr("width", w + p * 2)
//	    .attr("height", h + p * 2)
	  .append("svg:g")
	  .attr("transform", "translate(" + transX + "," + transY + ")");

// DS- Experiment: works, but transformation messes up. To investigate.
//	$C({element: "g", transform: "translate(" + transX + "," + transY + ")", appendTo: vis.node()})

	var rules = vis.selectAll("g.rule")
	    .data(x.ticks(10))
	  .enter().append("svg:g")
	    .attr("class", "rule");
	
	rules.append("svg:line")
	    .attr("x1", x)
	    .attr("x2", x)
	    .attr("y1", 0)
	    .attr("y2", h - 1);
	
	rules.append("svg:line")
	    .attr("class", function(d) { return d ? null : "axis"; })
	    .attr("y1", y)
	    .attr("y2", y)
	    .attr("x1", 0)
	    .attr("x2", w + 1);
	
	rules.append("svg:text")
	    .attr("x", x)
	    .attr("y", h + 3)
	    .attr("dy", ".71em")
	    .attr("text-anchor", "middle")
	    .text(x.tickFormat(10));
	
	rules.append("svg:text")
	    .attr("y", y)
	    .attr("x", -3)
	    .attr("dy", ".35em")
	    .attr("text-anchor", "end")
	    .text(y.tickFormat(10));
	
	vis.append("svg:path")
	    .attr("class", "area")
	    .attr("d", d3.svg.area()
	    .x(function(d) { return x(d.x); })
	    .y0(h - 1)
	    .y1(function(d) { return y(d.y); }));
	
	vis.append("svg:path")
	    .attr("class", "line")
	    .attr("d", d3.svg.line()
	    .x(function(d) { return x(d.x); })
	    .y(function(d) { return y(d.y); }));
	
	vis.selectAll("circle.area")
	    .data(data)
	  .enter().append("svg:circle")
	    .attr("class", "area")
	    .attr("cx", function(d) { return x(d.x); })
	    .attr("cy", function(d) { return y(d.y); })
	    .attr("r", 3.5);
};


areaWin.build({
	x: 120,
	y: 120,
	width: 524,
	height: 420,
	minimized: true,
	contains: function() {return this.contents()}
});


