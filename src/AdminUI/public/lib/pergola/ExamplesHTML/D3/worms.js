
// DS: based on http://bl.ocks.org/1216850 by Jon Frost.

var worms = new pergola.Window("George And Albertina Baby Space Worms");

worms.build({
  isFull : true,
  fill : "black",
  contains : function() {
	  var
			repCountSpace = 80,
      mouse = {x : 400, y : 400},
      color = d3.scale.linear()
        .domain([0, repCountSpace])
        .interpolate(d3.interpolateHsl)
        .range(["hsl(250,0%,100%)", "hsl(180,100%,50%)"]),
      doc = this.doc,
			node = doc.transformable,
  		gGrad = $C({element : "linearGradient", id : "gGrad", x1 : "0%", y1 : "20%", x2 : "20%", y2 : "100%", appendTo : node}),
			aGrad = $C({element : "linearGradient", id : "aGrad", x1 : "0%", y1 : "20%", x2 : "20%", y2 : "100%", appendTo : node});

	  $C({element : "stop", offset : "20%", "stop-color" : "#8080FF", appendTo : gGrad});
	  $C({element : "stop", offset : "50%", "stop-color" : "blue", appendTo : gGrad});
	  $C({element : "stop", offset : "100%", "stop-color" : "#FFFF60", appendTo : gGrad});

	  $C({element : "stop", offset : "20%", "stop-color" : "#FF80FF", appendTo : aGrad});
	  $C({element : "stop", offset : "50%", "stop-color" : "purple", appendTo : aGrad});
	  $C({element : "stop", offset : "100%", "stop-color" : "#FFFF60", appendTo : aGrad});

	  this.george = d3.select(
	    $C({element : "g", "stroke-width" : 5, "stroke-opacity" : .2, fill : "url(#gGrad)", appendTo : node})
	  )
	
	  this.george.elements = this.george.selectAll()
	      .data(d3.range(repCountSpace))
	    .enter().append("svg:ellipse")
	      .attr("rx", function(d) { return repCountSpace - d; })
	      .attr("ry", function(d) { return ((repCountSpace - d) * .66); })
	      .attr("stroke", function(d) { return color(d); })
	      .map(function(d) { return {center: [250, 250], angle: 30}; });
	
	  this.george.timer = pergola.Timer()
	  .initialize({
	    handle : this,
	    callback : excite,
	    frequence : 25,
	    target : this.george.elements,
	    horOffset : -110,
	    count : 0
	  });
	
	  this.albertina = d3.select(
	    $C({element : "g", "stroke-width" : 48, "stroke-opacity" : .04, fill : "url(#aGrad)", appendTo : node})
	  );
	
	  this.albertina.elements = this.albertina.selectAll()
	      .data(d3.range(repCountSpace))
	    .enter().append("svg:ellipse")
	      .attr("rx", function(d) { return repCountSpace - d; })
	      .attr("ry", function(d) { return ((repCountSpace - d) * .66); })
	      .attr("stroke", function(d) { return color(d); })
	      .map(function(d) { return {center: [250, 250], angle: 30}; });
	
	  this.albertina.timer = pergola.Timer()
	  .initialize({
	    handle : this,
	    callback : excite,
	    frequence : 25,
	    target : this.albertina.elements,
	    horOffset : 110,
	    count : 0
	  });
	
	  function excite (timer) {
	    timer.count ++;
	    timer.target.attr("transform", function(d, i) {
	      d.center[0] = (d.center[0] + ((mouse.x + timer.horOffset - d.center[0]) / (i + 10)));
	      d.center[1] = (d.center[1] + ((mouse.y - d.center[1]) / (i + 10)));
	      d.angle = (d.angle + (Math.sin((timer.count + i) / 10) * 3));
	      return "translate(" + d.center + ") rotate(" + d.angle + ")";
	    });
	  };

	  this.registerEvents(doc.container, "mouseover", function (evt) {
//	    pergola.dragarea.resize(doc.absoluteX(doc.container), doc.absoluteY(doc.container), doc.visibleW, doc.visibleH)
// workaround for Firefox getCTM() bug (https://bugzilla.mozilla.org/show_bug.cgi?id=873106)
	    pergola.dragarea.resize(worms.x + doc.x, worms.y + doc.y, doc.visibleW, doc.visibleH)	    
			.activate({
	      handle : worms,
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

















