
/*
 *			 PERGOLA CONFIGURATION FILE
 *
 * template file 
 *
 * After runtime these variables are no longer used  
*/


"use strict"

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
path = "pergola/",

/* 
 * Existing property name in pergola.skins (skins.js)
*/
skin = "office",

/*
 * Color keyword name, custom color name, or color in any legal format.
*/
theme = "cube",

undos = 16,

/*
 * Determines whether pergola has a system menu on the taskbar.
*/
systemMenu = true,

/*
 * system taskbar position ("top"; "bottom").
*/
taskbarPosition = "top",

/*
 * Logo on taskbar.
*/
taskbarLogo = false,

/*
 * Number of decimal places
*/
decimals = 3,

/*
 * true, false, or "extended" (extended needs 1600px screen resolution).
*/ 
colorpicker = false,

/*
 * true, false, or number of lines (defaults to 20).
*/
debug = false;
