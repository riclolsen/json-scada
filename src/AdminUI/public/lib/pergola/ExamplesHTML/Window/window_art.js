
//=======================   Pergola examples - Windows - ellipsis   ==========================



var window1 = new pergola.Window()
.build({
  x : 60,
  y : 60,
  width : 600,
  height : 400,
  fill : "#FFFFF4",
  hasToolBar : false,
  hasZoomAndPan : false,
});





var window2 = new pergola.Window("Ellipsis")
.build({
  x : 80,
  y : 80,
  width : 640,
  height : 480,
  contains : ellipses.dum(),
  menu : {
    menu1 : {
      title : "Menu 1",
      items : {
        item1 : {
          string : "Menu Item #1",
          active : true,
          check : true,
          fn : myMessage
        },
        item2 : {
          string : "Menu Item #2",
          active : true,
          check : true,
          fn : 'myMessage'
        },
        item3 : {
          string : "Menu Item #3",
          active : true,
          fn : 'myMessage'
        }
      }
    },
    menu2 : {
      title : "Menu 2",
      items : {
        item1 : {
          string : "Menu Item #1",
          active : true,
          check : true,
          fn : 'myMessage'
        },
        item2 : {
          string : "Menu Item #2",
          active : true,
          check : true,
          fn : 'myMessage'
        },
        item3 : {
          string : "Menu Item #3",
          active : true,
          fn : 'myMessage',
          separator : new pergola.Separator()
        },
        item4 : {
          string : "Menu Item #4",
          active : true,
          fn : 'myMessage'
        },
        item5 : {
          string : "Menu Item #5",
          active : true,
          fn : 'myMessage'
        }
      }
    }
  }
});




/*
 * Several ways of defining and referencing User Functions (see the doc).
 * Shown:
 * top level function;
 * prototype method;
 * instance method for item2 in menu1, overriding the prototype method.
 *
*/

function myMessage() {
  $M('You clicked ' + this.string);
}

pergola.MenuItem.prototype.myMessage = function (evt) {
  $M('You clicked ' + this.string);
}

window2.menu.menu1.list.item2.myMessage = function (evt) {
  $M('You clicked ' + this.string);
}
