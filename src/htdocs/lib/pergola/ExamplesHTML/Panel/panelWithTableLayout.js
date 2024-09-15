
//=============================   Pergola examples - panel with table layout   ==========================



/*
 * The tab's "contains" property (function or elements) appends contents to the tab's pane. if we use a function
 * we have access to the tab's and panel's geometrical -and all other- properties for our tab contents layout.
 * Note that the functions referenced by the 'fn' properties of the tabs are invoked when clicking the panel's
 * OK button. Functions and event handling defined in tab's contents will have instead real time effect.
 *
 * 
*/

var myPanel = new pergola.Panel("My panel")
.build({
  type : "dialog",
  title : "PANEL WITH TABLE LAYOUT",
  x : 100,
  y : 50,
  width : 628,
  height : 440,
  okButton : {
    text : "OK"                  // text is most often configurable, but not for DialogButton. Don't assign a text object here
  },
  cancelButton : {
    text : "Close"
  },
  isOpen : true,             // panels are mostly used for dialogs. Default is false
  layout : {
    type : "table",              // the name of a Layout prototype method
    rows : 3,
    cols : 3,
    spacing : 4,                 // Number - user space units
    attributes : {               // Any <rect> attributes go here
//      fill : "#F8F8F8",          // Default white 
      stroke : "#D0D0D0",
//      "stroke-width" : 2
    },
  },
  contains : function () {
    for (var c in this.cells) {
      $C({
        element : "text",
        x : this.layout.cellWidth / 2,
        y : this.layout.cellHeight / 2 + 8,
        "font-size" : "16pt",
        "font-weight" : "bold",
        fill : "#F0F0F0",
        "text-anchor" : "middle",
        textNode : "CELL " + c,
        appendTo : this.cells[c]
      });
    }
  }
});

