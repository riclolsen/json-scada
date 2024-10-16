
// DS: based on http://bl.ocks.org/1216850 by Jon Frost.

var ray = new pergola.Window("Ray")
.build({
  isFull : true,
  fill : "black",
//  docBgResizable : false,
  contains : function() {
	  var
			repCountSpace = 80,
      mouse = {x : 440, y : 200},
      color = d3.scale.linear()
        .domain([0, repCountSpace])
        .interpolate(d3.interpolateHsl)
        .range(["hsl(180,0%,50%)", "hsl(200,20%,50%)"]),
      doc = this.doc,
	  	gradient = $C({element : "linearGradient", id : "ray-grad", x1 : "50%", y1 : "0%", x2 : "50%", y2 : "100%", appendTo : doc.transformable});

	  $C({element : "stop", offset : "25%", "stop-color" : "#441616", appendTo : gradient});
	  $C({element : "stop", offset : "50%", "stop-color" : "brown", appendTo : gradient});
	  $C({element : "stop", offset : "100%", "stop-color" : "white", appendTo : gradient});
	
	  this.ray = d3.select($C({element : "g", "stroke-width" : 8, "stroke-opacity" : .05, fill : "url(#ray-grad)", appendTo : doc.transformable}));
	
	  this.ray.elements = this.ray.selectAll()
	    .data(d3.range(repCountSpace))
	    .enter().append("svg:ellipse")
	    .attr("rx", function(d) { return (repCountSpace - d) * 5; })
	    .attr("ry", function(d) { return ((repCountSpace - d) * .4); })
	    .attr("stroke", function(d) { return color(d); })
	    .map(function(d) { return {center: [440, 200], angle: 0}; });
	
	  this.ray.elements[0].reverse();
		this.ray.timer = pergola.Timer()
	  .initialize({
	  	handle : this,
	    callback : function (timer) {
		    if (!isFinite(timer.count ++)) timer.count = 0;
		    timer.target.attr("transform", function(d, i) {
		      d.center[0] += ((mouse.x - d.center[0]) / (i + 10));
		      d.center[1] += ((mouse.y - d.center[1]) / (i + 10));
		      d.angle += (Math.sin((timer.count + i) / 10) * .5);
		      return "translate(" + d.center + ") rotate(" + (d.angle) + ")";
		    });
		  },
	    frequence : 40,
	    target : this.ray.elements,
	    count : 0
	  });

	  this.registerEvents(doc.container, "mouseover", function (evt) {
//	    pergola.dragarea.resize(doc.absoluteX(doc.container), doc.absoluteY(doc.container), doc.visibleW, doc.visibleH)
// workaround for Firefox getCTM() (https://bugzilla.mozilla.org/show_bug.cgi?id=873106)
	    pergola.dragarea.resize(ray.x + doc.x, ray.y + doc.y, doc.visibleW, doc.visibleH)
			.activate({
	      handle : ray,
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

















