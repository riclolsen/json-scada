
//=============================   Pergola examples - Legend   ==========================





var legend1 = new pergola.Legend()
.build({
	x : 120,
	y : 110,
	vSpacing : 6,
	textAttributes : {
		"font-style" : "italic"
	},
	legend : {
		redLine : {
			art : {
				elements : [
					{element : "path", d : "M0,-2 h 60", fill : "none", stroke : "#F0F0F0", "stroke-width" : 4},
					{element : "path", d : "M0,-2 h 60", fill : "none", stroke : "#F80000", "stroke-width" : 4, "stroke-dasharray" : "9,8"}
				]
			},
			caption : "The red line"
		},
		greenLine : {
			art : {
				elements : [
					{element : "path", d : "M0,-1.5 h 60", fill : "none", stroke : "green", "stroke-width" : 3}
				]
			},
			caption : "The green line"
		},
		"The blue line" : {
			art : {
				elements : [
					{element : "path", d : "M0,-2 h 60", fill : "none", stroke : "#0060FF", "stroke-width" : 2}
				]
			},
			caption : "The blue line"
		},
		"The technicolor line" : {
			art : {
				image : {
					url : pergola.path + "lib/palettes/legendSpectrum.png",
					width : 60,
					height : 4
				}
			},
			caption : "The artwork can be one or more primitives, or an image"
		}
	}
});



var legend2 = new pergola.Legend()
.build({
	x : 120,
	y : 260,
	textOffset : 20,
	vSpacing : 12,
	textAttributes : {
		"font-size" : "11px",
		"font-weight" : "bold",
		fill : "#3040C0"
	},
	legend : {
		"The red line" : {
			art : {
				elements : [
					{element : "path", d : "M0,-2 h 60", fill : "none", stroke : "#F0F0F0", "stroke-width" : 4},
					{element : "path", d : "M0,-2 h 60", fill : "none", stroke : "#F80000", "stroke-width" : 4, "stroke-dasharray" : "9,8"}
				]
			},
			caption : "The red line"
		},
		"The green line" : {
			art : {
				elements : [
					{element : "path", d : "M0,-1.5 h 60", fill : "none", stroke : "green", "stroke-width" : 3}
				]
			},
			caption : "The green line"
		},
		"The blue line" : {
			art : {
				elements : [
					{element : "path", d : "M0,-2 h 60", fill : "none", stroke : "#0060FF", "stroke-width" : 2}
				]
			},
			caption : "The blue line"
		},
		"The technicolor line" : {
			art : {
				image : {
					url : pergola.path + "lib/palettes/legendSpectrum.png",
					width : 60,
					height : 4
				}
			},
			caption : [
				"The caption can have more than one line, and you can also",
				"specify text attributes using SVG vocabulary and grammar"
			]
		}
	}
});




new pergola.Legend()
.build({
	x : 180,
	y : 430,
	vSpacing : 12,
	textAttributes : {
		"font-size" : "13px"
	},
	legend : {
		item1 : {
			caption : 'Vertical spacing and text offset can be overridden.'
		},
		item2 : {
			caption : 'The class has toggleOn(), toggleOff() and move() prototype methods,'
		}
	}
});

