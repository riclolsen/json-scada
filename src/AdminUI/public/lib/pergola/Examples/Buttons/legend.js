


var legend = new pergola.Legend()
.build({
	x : 140,
	y : 380,
	vSpacing : 9,
	legend : {
		item1 : {
			caption : [
				"The basic button is a rectangle and has two preset sizes.",
				"All of its properties can be overridden."
			]
		},
		item2 : {
			caption : "You can define your own shape or use a shape from the library."
		},
		item3 : {
			caption : "The user define shape can be any SVG primitive shape."
		},
		item4 : {
			caption : "You can add any property as element attribute using SVG vocabulary."
		},
		item5 : {
			caption : "You can easily change the fill (hover) and/or the mask fill."
		},
		item6 : {
			caption : "You can define the tranform property using SVG grammar."
		},
		item7 : {
			caption : "Symbols can be scaled, rotated, and filled to your needs."
		},
		item8 : {
			caption : [
				"You position a symbol by setting its x and y properties,",
				"or let the engine center it."
			]
		},
		item9 : {
			caption : [
				"You can place text and a symbol together in the same button.",
				"You can define any properties for the text property object",
				"as text element attributes using SVG syntax."
			]
		},
		item10 : {
			caption : [
				"You can also assign an image besides the fill property.",
				"Symbols can be artwork or images and you can set their opacity."
			]
		}
	},

	hilight : function(evt) {
		if (evt.type == "mouseup") {
	 		this.target.setAttributeNS(null, "fill", "#C00000");
	 	}
	}

});


