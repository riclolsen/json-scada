
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

