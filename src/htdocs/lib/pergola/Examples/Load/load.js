
//====================   Pergola examples - Load SVG Document   ==========================




var birds = new pergola.Window("BIRDS")
.build({
  x : 60,
  y : 60,
  fill : "#FFFFF0",
  menu : {
    palaeognathae : {
      title : "Palaeognathae",
      items : {
        item1 : {
          string : "Menu Item #1",
          check : true,
          fn : 'myMessage'
        },
        item2 : {
          string : "Menu Item #2",
          check : true,
          fn : 'myMessage'
        },
        item3 : {
          string : "Menu Item #3",
          fn : 'myMessage'
        }
      }
    },
    neognathae : {
      title : "Neognathae",
      items : {
        item1 : {
          string : "Menu Item #1",
          check : true,
          fn : 'myMessage'
        },
        item2 : {
          string : "Menu Item #2",
          check : true,
          fn : 'myMessage'
        },
        item3 : {
          string : "Menu Item #3",
          fn : 'myMessage',
          separator : new pergola.Separator()
        },
        item4 : {
          string : "Menu Item #4",
          fn : 'myMessage'
        },
        item5 : {
          string : "Menu Item #5",
          fn : 'myMessage'
        }
      }
    }
  },
  contains : function () {pergola.loadSVG({
      url : "docs/birds.svg",
      target : this,
      progress : this.progress
    });
  }
});






/*
 * User Functions
 * take several value types. See documentation.
 * The prototype and the instance method names below are both referenced as a string.
*/
birds.menu.palaeognathae.list.item2.myMessage = function (evt) {
  $M('You clicked ' + this.string);
}

pergola.MenuItem.prototype.myMessage = function (evt) {
  $M('You clicked ' + this.string);
}


