
//=============================   Pergola examples - simple panel   ==========================


$M("In the notification string you can use\nthe escape sequence \"\\n\" for new lines.", {x : 200, y : 120});





new pergola.Legend()
.build ({
	x : 100,
	y : 440,
	legend : {
		item1 : {
			caption : [
				"A pergola Message is preemptive,",
			  "but doesn't stop the script and can",
			  "replace the alert() in many cases."
			]
		}
	}
});


