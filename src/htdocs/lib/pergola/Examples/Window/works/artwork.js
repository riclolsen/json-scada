
/* 
 * Some contents for window2.
 * Window contents are appended to the group designated by childDoc.transformable.
 * Elements appended to any window element other than childDoc.transformable
 * will not be subject to the window's built-in transformation tools.
*/

var ellipses = {
	colors: ["teal", "crimson", "gold", "orange", "maroon", "violet", "forestgreen", "turquoise", "mediumblue"],
	group: $C({element:"g"}),
	dum: function() {
		var rx = 500;
		var ry = 200;
		for (var i = c = 0; i < 3; i++) {
		  for (var j = 0; j < 3; j++) {
		    var cx = rx + 200 + (rx * 2 + 200) * i;
		    var cy = ry + 200 + (ry * 2 + 200) * j;
		    $C({
					element: "ellipse", 
					cx: cx, 
					cy: cy, 
					rx: rx, 
					ry: ry, 
					fill: ellipses.colors[c++], 
					appendTo: ellipses.group
				});
		  }
		}
		return ellipses.group;
	},
	smart: function() {
		var rx = 500;
		var ry = 250;
		for (var i = c = 0; i < 3; i++) {
		  for (var j = 0; j < 3; j++) {
		    var cx = rx + 250 + (rx * 2 + 200) * j - 250 * (!c.isEven() && i == 1 && j == 0) + 250 * (!c.isEven() && i == 1 && j == 2);
		    var cy = ry + 500 + (ry * 2 + 500) * i - 250 * (!c.isEven() && i == 0) + 250 * (!c.isEven() && i == 2);
		    var angle = (45 + 45 * j) * (i == 0) + (45 + 45 * j) * (i == 2) * ~0X0;
		    $C({
					element: "ellipse", 
					cx: cx, 
					cy: cy, 
					rx: rx, 
					ry: (ry * (c != 4)) || rx, 
					fill: ellipses.colors[c++],
					transform: "rotate(" + angle + "," + cx + "," + cy + ")",
					appendTo: ellipses.group
				});
		  }
		}
		return ellipses.group;
	}
};
