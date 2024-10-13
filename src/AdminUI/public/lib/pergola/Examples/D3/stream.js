
//=======================   Pergola examples - D3 multiple documents - stream.js   =====================


var streamWin = new pergola.Window("D3 Stream");

/* DS-
 * Cleanest way to encapsulate in window:
 * Define a helper function as instance method for the "contains" property
 * of the streamWin object. The D3 example script is the body of the function.
*/
streamWin.contents = function() {
/* DS-
 * Turning some variables of the original D3 example into properties
 * of the window object to ensure global interactivity.
*/
  var n = 20, // number of layers
      m = 200, // number of samples per layer
      w = 960,
      h = 500,
      o = this,
      color = d3.interpolateRgb("#aad", "#556");

  this.data0 = d3.layout.stack().offset("wiggle")(stream_layers(n, m));
  this.data1 = d3.layout.stack().offset("wiggle")(stream_layers(n, m));
  this.data2 = d3.layout.stack().offset("wiggle")(stream_waves(n, m));

  this.transitFrom = this.data0;
  this.transitTo = this.data1;

  var mx = m - 1,
      my = d3.max(this.data0.concat(this.data1), function(d) {
        return d3.max(d, function(d) {
          return d.y0 + d.y;
        });
      });

  var area = d3.svg.area()
    .x(function(d) { return d.x * w / mx; })
    .y0(function(d) { return h - d.y0 * h / my; })
    .y1(function(d) { return h - (d.y + d.y0) * h / my; });

/* DS-
 * The doc.transformable group has DOM Events listeners. IE9 throws runtime errors 
 * with getBBox() for path elements appended without the "d" attribute (which is required).
 * Defer appending the group. 
*/
//  var vis = d3.select(this.doc.transformable)
//    .append("svg:g")
//    .attr("transform", "translate(" + 10 + "," + 40 + ")");

  var vis = d3.select($C({element : "g", transform : "translate(" + 10 + "," + 40 + ")"}));

  this.chart = vis.selectAll()
    .data(this.data0)
    .enter().append("svg:path")
    .attr("fill", function() { return color(Math.random()); })
    .attr("d", area);

  this.doc.transformable.appendChild(vis.node());

  this.transition = function (destination, fill) {
//    $D({"fill": fill, "shade": fill.darken()});
    color = fill ? d3.interpolateRgb(fill, fill.darken()) : color;
    this.chart.data(function() {
      o.transitTo = o.transitFrom;
      return o.transitFrom = destination;
    })
    .transition()
    .duration(2500)
    .attr("fill", function() { return color(Math.random()); })
    .attr("d", area);
  }
};


streamWin.build({
  x : 100,
  y : 100,
  width : 600,
  height : 420,
  menu : {
    transitions : {
      title : "Transitions",
      items : {
        streamLayers : {
          string : "Stream Layers",
          active : true,
          check : true,
          exclusive : true,
          fn : function () {
            streamWin.transitTo = (streamWin.transitFrom == streamWin.data0) ? streamWin.data1 : streamWin.data0;
          }
        },
        streamWaves : {
          string : "Stream Waves",
          active : true,
          check : false,
          exclusive : true,
          fn : function () {streamWin.transitTo = streamWin.data2;}
        }
      }
    }
  },
  contains : function () {this.contents();}
})
.addTools({
  streamGroup : {
    separator : true,
    transit : {
      symbol : {
        symbol : pergola.symbols.transition,
        x : 6,
        y : 11
      },
      quickTip : {tip : "Apply transition"},
      ev : "mouseup",
      fn : function (evt) {this.owner.transition(this.owner.transitTo);}
    }
  }
});


