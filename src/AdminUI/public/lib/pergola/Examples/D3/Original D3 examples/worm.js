
var wormWin = new pergola.Window("D3 Worm");


wormWin.contents = function() {

  var mouse = [480, 250],
    count = 0,
    w = 1000,
    h = 600,
    repCountTunnel = 300,
    repCountSpace = 130;

//var color = d3.scale.linear().domain([0,repCountSpace]).interpolate(d3.interpolateRgb).range(['green', 'blue']); 
//var color = d3.interpolateRgb("#fff", "#f00"); 
//var color = d3.interpolateHsl("hsl(210,100%,0%)", "hsl(180,100%,100%)");
  var color = d3.scale.linear().domain([0,repCountSpace]).interpolate(d3.interpolateHsl).range(["hsl(250,100%,50%)", "hsl(180,100%,50%)"]);
  
  var svg = d3.select("#svg-replicate").append("svg:svg")
      .attr("viewBox", "0 0 800 600")
      .attr("preserveAspectRatio", "xMinYMin meet");
  
  var gradient = svg.append("svg:defs")
    .append("svg:linearGradient")
      .attr("id", "gradient")
      .attr("x1", "0%")
      .attr("y1", "20%")
      .attr("x2", "20%")
      .attr("y2", "100%");
  gradient.append("svg:stop")
      .attr("offset", "20%")
      .attr("stop-color", "green");
  gradient.append("svg:stop")
      .attr("offset", "50%")
      .attr("stop-color", "blue");
  gradient.append("svg:stop")
      .attr("offset", "100%")
      .attr("stop-color", "orange");
      
  // Code for static centroid created used principles of the SVG-Replicate project.
  var tunnel = svg.append("svg:g")
      .attr("transform", "translate(" + w / 2.5 + "," + h / 3 + ")");
  
  tunnel.selectAll(".tunnel")
      .data(d3.range(0, repCountTunnel, 1))
    .enter().append("svg:circle")
      .attr("class", "tunnel")
      .attr("r", function(d) { return d / 2 + 5})
      .attr("cx", -200)
      .attr("cy", -100)
      //.attr("stroke", color)
      .attr("stroke",   function(d){ return color(d); })
      .attr("transform", function(d) {
        return "rotate(" + d/5 + ")"
            + "translate(" + d + "," + 0 + ")"
            + "rotate(0)";
      });
  
  // Code for friendly space worm, contained within an SVG group.
  var g = svg.selectAll(".space")
      .data(d3.range(repCountSpace))
    .enter().append("svg:g")
      .attr("class", "space")
      .attr("transform", "translate(" + mouse + ")");
      
  g.append("svg:ellipse")
      .attr("rx", 8)
      .attr("ry", 5)
      .attr("cx", 2)
      .attr("cy", 2)
      .attr("fill", "url(#gradient)")
  	.attr("stroke",   function(d){ return color(d); })
      .attr("transform", function(d, i) { return "scale(" + (1 - d / repCountSpace) * 10 + ")"; });
  
  g.map(function(d) {
    return {center: [250, 250], angle: 30};
  });
  
  svg.on("mousemove", function() {
    mouse = d3.svg.mouse(this);
  });
  
  d3.timer(function() {
    count += 1;
    g.attr("transform", function(d, i) {
      d.center[0] += (mouse[0] - d.center[0]) / (i + 5);
      d.center[1] += (mouse[1] - d.center[1]) / (i + 5);
      d.angle += Math.sin((count + i) / 10) * 3;
      return "translate(" + d.center + ")rotate(" + d.angle + ")";
    });
    return false; // Animation continues until return = true;
  });
}


wormWin.build({
  x : 100,
  y : 100,
  width : 600,
  height : 420,
  fill : "black",
  contains : function () {this.contents()}
});
