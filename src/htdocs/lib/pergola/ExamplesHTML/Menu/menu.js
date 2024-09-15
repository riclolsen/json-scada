
//=============================   Pergola examples - menus   ==========================


/*
 * Menus are not top level objects. They normally are properties of some object.
 * For this example we create an environment consisting of an object, its menubar,
 * its menus and a user function.
 *
 * See also the windows examples: the initial value of the 'hasMenuBar' prototype
 * property of the Window class is set to true and unless overridden, a window
 * instance is configured to have a 'menu' object property.
*/

var myObject = {
  menubar : new pergola.Menubar()
    .build({
      x : 180,
      y : 80
    }),
  menu : {
    food : new pergola.Menu("Food"),
    other : new pergola.Menu("Other")
  },
  message : function (evt) {
    var
      msg,
      inOrOut,
      toFrom;

    if (this.propertyIsEnumerable("check")) {
      if (this.check) {
        inOrOut = ' added ';
        toFrom = ' to ';
      }
      else {
        inOrOut = ' removed ';
        toFrom = ' from ';
      }

      msg = 'You' + inOrOut + this.string + toFrom + 'the basket';
    }
    else msg = 'You clicked ' + this.string;
    $M(msg, {x : 240, y : 88});
  }
};


/*
 * deferred construction of the menu components
*/
myObject.menu.food.build({
  owner : myObject,
  parent : myObject.menubar.container,
  title : myObject.menu.food.name,            // or any string
  items : {
    item1 : {
      string : "Menu Item #1",
      check : true,
      fn : myObject.message
    },
    item2 : {
      string : "Menu Item #2",
      check : false,
      fn : myObject.message
    },
    food : {
      string : "Food",
      submenu : {
        items : {
          organic : {
            string : "Organic (OOS)",
            active : false,
            fn : myObject.message
          },
          fruits : {
            string : "Fruits",
            submenu : {
              items : {
                bananas : {
                  string : "Bananas",
                  check : false,
                  fn : myObject.message
                },
                pears : {
                  string : "Pears",
                  check : false,
                  fn : myObject.message
                },
                cherries : {
                  string : "Cherries",
                  check : false,
                  fn : myObject.message
                },
                plums : {
                  string : "Plums",
                  check : false,
                  fn : myObject.message
                }
              }
            }
          },
          vegetables : {
            string : "Vegetables",
            submenu : {
              items : {
                carrots : {
                  string : "Carrots",
                  check : false,
                  fn : myObject.message
                },
                potatoes : {
                  string : "Potatoes",
                  check : false,
                  fn : myObject.message
                },
                beans : {
                  string : "Beans",
                  check : false,
                  fn : myObject.message
                },
                tomatoes : {
                  string : "Tomatoes",
                  check : false,
                  fn : myObject.message
                }
              }
            }
          }
        }
      }
    },
    item4 : {
      string : "Menu Item #4",
      fn : myObject.message
    }
  }
});


myObject.menu.other.build({
  owner : myObject,
  parent : myObject.menubar.container,
  title : myObject.menu.other.name,
  items : {
    item1 : {
      string : "Menu Item #1",
      check : true,
      fn : myObject.message
    },
    item2 : {
      string : "Menu Item #2",
      check : true,
      fn : myObject.message
    },
    item3 : {
      string : "Menu Item #3",
      fn : myObject.message,
      separator : new pergola.Separator()
    },
    item4 : {
      string : "Menu Item #4",
      fn : myObject.message
    },
    item5 : {
      string : "Menu Item #5",
      fn : myObject.message
    }
  }
});