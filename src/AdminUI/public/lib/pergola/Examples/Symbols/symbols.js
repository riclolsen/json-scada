
//=============================   Pergola examples - shapes   ==========================



var
	symbols = pergola.symbols,
	symbol,
	cols = 12,
	x = hSpace = 60,
	y = vSpace = 80,
	g = $C({element : "g", transform : "translate(10 0)", appendTo : pergola.user});

for (var a in symbols) {
	symbol = symbols[a];
	if (symbol.length) make(symbol);
	else recurse(symbol);
}

function recurse (symbol) {
	var a, s;

	for (a in symbol) {
		s = symbol[a];
		if (!s.length) recurse(s);
		else make(s);
	}
}

function make (symbol) {
	pergola.symbol({
	  symbol : symbol,
	  x : x,
	  y : y,
	  parent : g
	});
	x += hSpace;
	if (x > hSpace * cols) {
		x = hSpace;
		y += vSpace;
	}
}











new pergola.Legend()
.build({
	x : 42,
	y : 460,
	legend : {
		item1 : {
			caption : [
				"A symbol is an array of one or more elements with their paint and geometrical attributes.",
				"When using a symbol you can override its attributes or define new ones, using SVG attributes vocabulary.",
				"You can attach a symbol to objects like buttons, tools, etc., or use it as standalone to create",
				"icons by invoking the pergola.symbol() utility function."
			]
		}
	}
});
