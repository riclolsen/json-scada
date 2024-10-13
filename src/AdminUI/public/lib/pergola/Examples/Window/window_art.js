
//=======================   Pergola examples - Windows - ellipsis   ==========================



var window1 = new pergola.Window("Window 1")
.build({
  fill : "#FFFFF0",
  hasStatus : false
});





var window2 = new pergola.Window("Artwork")
.build({
  x : 240,
  y : 200,
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




// How to hide the taskbar
pergola.taskbar.toggleOff();




