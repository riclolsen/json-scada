/*
 *			 PERGOLA CONFIGURATION FILE
 *
 * After runtime these variables are no longer used  
*/

var
/*
 * 0 for standalone SVG context, or ID (string) of HTML element (e.g. container="mySVG").
 * If svg goes in an iframe or div with overflow: auto, set svgWidth and svgHeight (default 100%).
*/ 
container = 0,
// svgWidth=0
// svgHeight=0

/*
 * Path to the Pergola folder 
*/
path = "../../pergola/",

/* 
 * Existing property name in pergola.skins (skins.js)
*/
skin = "office",

/*
 * Color keyword name, custom color name, or color in any legal format.
*/
theme = "cube",

/*
 * Determines whether pergola has a system menu on the taskbar.
*/
systemMenu=false,

/*
 * system taskbar position ("top"; "bottom").
*/
taskbarPosition="top",

/*
 * Logo on taskbar.
*/
taskbarLogo=false,


undos=16,

/*
 * Number of decimal places
*/
decimals=3,


/*
 * true, false, or number of lines (defaults to 20).
*/
debug=false;
