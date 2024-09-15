
//=============================   Pergola examples - panel with tabs and layout   ==========================




var myPanel = new pergola.Panel("myPanel");


/*
 * The tab's "contains" property (function or elements) appends contents to the tab's pane. if we use a function
 * we have access to the tab's and panel's geometrical -and all other- properties for our tab contents layout.
 * Note that the functions referenced by the 'fn' properties of the tabs are invoked when clicking the panel's
 * OK button. Functions and event handling defined in tab's contents will have instead real time effect.
 *
 * 
 * Pergola classes allow both method chaining and deferred construction.
 * Showing an example of deferred construction and its advantages:
 *  
 * - opportunity to define properties between definition and construction
 * - opportunity to place the call elsewhere in the structural organization
 * - dynamic or conditional execution
 * - self-reference capability.
*/


myPanel.populate = {

  "table Layout" : function () {
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
  },

  "tab # 2" : function () {
    $C({
      element : "text",
      x : this.owner.width / 2,
      y : 200,
      "font-size" : 120,
      "font-weight" : "bold",
      fill : "#F0F0F0",
      "text-anchor" : "middle",
      textNode : this.title.toUpperCase(),
      appendTo : this.pane.container
    });
  },

  "HTML table" : function () {
    var switchTag = $C({
          element : "switch",
          appendTo : this.pane.container
        }),
        fObj = $C({
          element : "foreignObject",
          x : 9,
          width : this.pane.width,
          height : this.pane.height,
          appendTo : switchTag
        }),
        body = pergola.HTML_FO({
          element : "body",
          xmlns : pergola.ns.xhtml,
          appendTo : fObj
        }),
        table = pergola.HTML_FO({
          element : "table",
          border : 1,
          cellpadding : 2,
          cellspacing : 2,
          style : "width: 100%; background-color: #F8F8F8;",
          appendTo : body
        });

    for (var i = 0; i < 3; i++) {
      var tr = pergola.HTML_FO({
            element : "tr",
            appendTo : table
          });

      for (var j = 0; j < 3; j++) {
        var td = pergola.HTML_FO({
              element : "td",
              style : "height: " + parseInt((this.pane.height - 40) / 3) + "px; text-align: center;",
              appendTo : tr
            });

        pergola.HTML_FO({
          element : "p",
          style : "font-size: 16pt; font-weight: bold; color: #F0F0F0;",
          textNode : ("CELL " + (i * 3 + j)),
          appendTo : td
        });
      }
    }
    $C({
      element : "text",
      x : this.pane.width / 2,
      y : 80,
      "font-size" : 16,
      "text-anchor" : "middle",
      textNode : "<foreignObject> is not implemented in Internet Explorer Trident",
      appendTo : switchTag
    });
  }
};



myPanel.build({
  type : "dialog",
  title : "PANEL WITH TABS",
  x : 98,
  y : 12,
  width : 600,
  height : 440,
  margin : 0,
  okButton : {text : "OK"},
  cancelButton : {text : "Cancel"},
  isOpen : true,             // panels are mostly used for dialogs. Default is false
  fn : function () {
    alert ("Calling Panel tabs User Functions...")
    for (var t in this.tabs) this.tabs[t].fn();
  },
  layout : {
    type : "tabbed",                 // the name of a Layout prototype method
    tabs : {
      "table Layout" : {
        active : true,
        title : "table Layout",
        layout : {
          type : "table",                    // the name of a Layout prototype method
          rows : 3,
          cols : 3,
          spacing : 4,                       // Number - inherited units only
          attributes : {                     // Any <rect> attributes go here
//            fill : "#F8F8F8",          // Default white 
            stroke : "#D0D0D0",
//            "stroke-width" : 2
          }
        },
        contains : myPanel.populate["table Layout"],
        fn : function () {
          alert('Tab \"table Layout\" User Function calls the legend toggleOff() method.');
          legend.toggleOff();
        }
      },
      "tab # 2" : {
        title : "tab # 2",
        contains : myPanel.populate["tab # 2"],
        fn : function () {alert('Tab \"tab # 2\" User Function only shows this alert');}
      },
      "HTML table" : {
        title : "HTML table",
        contains : myPanel.populate["HTML table"],
        fn : function () {alert('Tab \"HTML table\" User Function only shows this alert')}
      }
    }
  }
});




var legend = new pergola.Legend()
.build({
  x : 20,
  y : 470,
  "font-size" : 11,
  legend : {
    item1 : {
      caption : [
        "The html table (foreignObject) in the \"HTML table\" tab works for:",
        "\u2001\u2022 Firefox",
        "\u2001\u2022 Opera",
        "\u2001\u2022 Chrome",
        "\u2001\u2022 Safari (partially)"
      ]
    },
    item2 : {
      caption : [
        "The foreignObject element is not implemented in:",
        "\u2001\u2022 IE Trident",
        "\u2001\u2022 ASV"
      ]
    }
  }
});




