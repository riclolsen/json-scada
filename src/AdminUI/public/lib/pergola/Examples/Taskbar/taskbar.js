
//=============================   pergola examples - taskbar   ==========================



pergola.taskbar.toggleOn();


new pergola.Legend()
.build({
	x : 120,
	y : 120,
	vSpacing : 12,
	legend : {
		item1 : {
			caption : [
				'A taskbar can be positioned top or bottom, you can override its initial display, and you can toggle its display.'
			]
		},
		item2 : {
			caption : [
				'The size and look&feel are configurable.'
			]
		},
		item3 : {
			caption : [
				'The "systemMenu" variable in the config file determines if the system taskbar has a system menu, and the ',
        '"taskbarLogo" variable let\'s you place your logo. These determine the layout of other objects on the taskbar.'
			]
		}
	}
});
