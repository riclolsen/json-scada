
// DS: based on http://bl.ocks.org/1216850 by Jon Frost.

var phoenix = new pergola.Window("The Phoenix")
.build({
  isFull : true,
  fill : "black",
  contains : function() {
	  var
			repCountSpace = 120,
	    mouse = {x : 400, y : 400},
	    color = d3.scale.linear()
	      .domain([0, repCountSpace])
	      .interpolate(d3.interpolateHsl)
	      .range(["hsl(180,50%,75%)", "hsl(315,100%,45%)"]),
	    doc = this.doc,
	  	gradient = $C({element : "linearGradient", id : "phoenix-grad", x1 : "0%", y1 : "20%", x2 : "20%", y2 : "100%", appendTo : doc.transformable});

	  $C({element : "stop", offset : "20%", "stop-color" : "#FFFF80", appendTo : gradient});
	  $C({element : "stop", offset : "50%", "stop-color" : "#8080FF", appendTo : gradient});
	  $C({element : "stop", offset : "80%", "stop-color" : "orange", appendTo : gradient});
	
	  this.phoenix = d3.select(
	    $C({element : "g", "stroke-width" : 24, "stroke-opacity" : .1, "fill-opacity" : .1, fill : "url(#phoenix-grad)", appendTo : doc.transformable})
//      $C({element : "g", "stroke-width" : 20, "stroke-opacity" : .1, "fill-opacity" : .1, fill : "#FFFFFF", appendTo : doc.transformable})
	  );
	
	  this.phoenix.elements = this.phoenix.selectAll()
	    .data(d3.range(repCountSpace))
	    .enter().append("svg:ellipse")
	    .attr("rx", function(d) { return (repCountSpace - d) * 2.25; })
	    .attr("ry", function(d) { return ((repCountSpace - d) * .2); })
	    .attr("stroke", function(d) { return color(d); })
	    .map(function(d) { return {center: [250, 250], angle: 0}; });
	
	  this.phoenix.elements[0].reverse();
		this.phoenix.timer = pergola.Timer()
	  .initialize({
	    handle : this,
	    callback : excite,
	    frequence : 25,
	    target : this.phoenix.elements,
	    count : 0
	  });
	
	  function excite (timer) {
	    if (!isFinite(timer.count ++)) timer.count = 0;
	    timer.target.attr("transform", function(d, i) {
	      d.center[0] += ((mouse.x - d.center[0]) / (i + 10));
	      d.center[1] += ((mouse.y - d.center[1]) / (i + 10));
	      d.angle += (Math.sin((timer.count + i) / 10) * 4);
	      return "translate(" + d.center + ") rotate(" + (d.angle) + ")";
	    });
	  };

	  this.registerEvents(doc.container, "mouseover", function (evt) {
//	    pergola.dragarea.resize(doc.absoluteX(doc.container), doc.absoluteY(doc.container), doc.visibleW, doc.visibleH)
// workaround for Firefox getCTM() bug (https://bugzilla.mozilla.org/show_bug.cgi?id=873106)
	    pergola.dragarea.resize(phoenix.x + doc.x, phoenix.y + doc.y, doc.visibleW, doc.visibleH)	    
			.activate({
	      handle : phoenix,
	      callback : function (evt) {
	        var
						point = pergola.mousePoint(evt),
	      		offset = doc.getOffset();
	
					mouse.x = (point.x - offset.x) / doc.scaleFactor;
	        mouse.y = (point.y - offset.y) / doc.scaleFactor;
	      },
	      updateCoordinates : false
	    });
	  });
	}
});

















