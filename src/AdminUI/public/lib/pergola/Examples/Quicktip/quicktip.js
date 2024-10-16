
//=============================   Pergola examples - quick tip   ==========================



var emptyTool = new pergola.ToolButton()
.build({
  x : 240,
  y : 200,
  width : 28,
  height : 22,
  stroke : "sandybrown",
  extra : {
    rx : 6
  },
  quickTip : {
    tip : "Quick–tips can be static or mouse–tracking. \nThe pop–up delay is configurable globally \nand can be overridden. A quick–tip can be a \nreference to the library, or can also be defined \non the fly, either as a string (with LF escape \nsequence \u005cn) or as an array. \n\nMore info in the documentation."
  }
});


var spy = new pergola.ToolButton()
.build({
  x : 340,
  y : 200,
  width : 38,
  height : 22,
  text : {
    x : 5,
    y : 18,
    'font-size' : 18,
    fill : "white",
    textNode : "007",
    'pointer-events' : "none"
  },
  textFillInverse : "#D00000",
  quickTip : {
    tip : [
      "My name is Bond,",
      "James Bond"
    ],
    x : 378,                   // Fixed position (no mouse tracking)
    y : 168,
    delay : 250                // Override default pop up "delay" (700 ms)
  }
});


/*
 * Alternate formats (see the documentation):
 *
 * 1) String:
 * if the string references a property of the pergola.quicktip object (qtips.js file)
 * that quicktip is used, otherwise the string is the text of the quicktip. Use the escape sequence \n for new line.
 *
 * 2) Array of strings.
*/
