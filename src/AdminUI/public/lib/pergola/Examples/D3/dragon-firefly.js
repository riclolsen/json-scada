
// DS: based on http://bl.ocks.org/1216850 by Jon Frost.

var bug = new pergola.Window("Dragon-firefly")
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
	  	gradient = $C({element : "linearGradient", id : "bug-grad", x1 : "0%", y1 : "50%", x2 : "100%", y2 : "50%", appendTo : doc.transformable});

	  $C({element : "stop", offset : "0%", "stop-color" : "blue", "stop-opacity" : .1, appendTo : gradient});
	  $C({element : "stop", offset : "50%", "stop-color" : "yellow", "stop-opacity" : .1, appendTo : gradient});
	  $C({element : "stop", offset : "100%", "stop-color" : "red", "stop-opacity" : .1, appendTo : gradient});
	
	  this.bug = d3.select(
	    $C({element : "g", "stroke-width" : 1, "stroke-opacity" : .1, fill : "url(#bug-grad)", appendTo : doc.transformable})
	//    $C({element : "g", "stroke-width" : 24, "stroke-opacity" : .1, "fill-opacity" : .1, fill : "#FFFFC0", appendTo : doc.transformable})
	  );
	
	  this.bug.elements = this.bug.selectAll()
	    .data(d3.range(repCountSpace))
	    .enter().append("svg:ellipse")
	    .attr("rx", function(d) { return (repCountSpace - d) * 1.5; })
	    .attr("ry", function(d) { return ((repCountSpace - d) * .2); })
	    .attr("stroke", function(d) { return color(d); })
	    .map(function(d) { return {center: [250, 250], angle: 0}; });
	
	  this.bug.elements[0].reverse();
		this.bug.timer = pergola.Timer()
	  .initialize({
	    handle : this,
	    callback : excite,
	    frequence : 10,
	    target : this.bug.elements,
	    count : 0
	  });
	
	  function excite (timer) {
	    if (!isFinite(timer.count ++)) timer.count = 0;
	    timer.target.attr("transform", function(d, i) {
	      d.center[0] += ((mouse.x - d.center[0]) / (i + 10));
	      d.center[1] += ((mouse.y - d.center[1]) / (i + 10));
	      d.angle += (Math.sin((timer.count + i) / 10) * 12);
	      return "translate(" + d.center + ") rotate(" + (d.angle) + ")";
	    });
	  };
	
	  this.registerEvents(doc.container, "mouseover", function (evt) {
//	    pergola.dragarea.resize(doc.absoluteX(doc.container), doc.absoluteY(doc.container), doc.visibleW, doc.visibleH)
// workaround for Firefox getCTM() bug (https://bugzilla.mozilla.org/show_bug.cgi?id=873106)
	    pergola.dragarea.resize(bug.x + doc.x, bug.y + doc.y, doc.visibleW, doc.visibleH)
			.activate({
	      handle : bug,
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

















