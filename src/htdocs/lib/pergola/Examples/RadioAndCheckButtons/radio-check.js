
//=============================   Pergola examples - radio and check buttons   ==========================



/*
 * For a group of RadioButton instances you define an "owner" object (in this example the instance property "radio"
 * of the panel, which acts as a placeholder). The properties "options" and "selection" are dynamically added to
 * manage the state of the buttons. User Functions can be assigned to the buttons and/or to the "owner" object.
 *
 * For a group of CheckBox instances you can define an "owner" object. The property "checked" (array) is
 * dynamically added to manage the state of the check-boxes. User Functions can be assigned to the buttons
 * and/or to the "owner" object.
*/

var optionsPanel = new pergola.Panel("Options Panel")
.build({
  type : "dialog",
  title : "OPTIONS",
  x : 200,
  y : 100,
  width : 300,
  height : 240,
  isOpen : true,
  radio : {
    selection : null,
    fn : function () {$M(this.selection.caption.element.firstChild.data + " is selected.", {x : 300, y : 200});}
  },
  check : {
    fn : function () {
    	var string = this.checked.length ? "These check boxes are checked:\n" : "No check boxes are checked";    

      if (this.checked.length) {
        for (var a in this.checked) string += this.checked[a].caption.element.firstChild.data + "\n";
      }

      $M(string, {x : 300, y : 200});
    }
  }
});



var radioButton1 = new pergola.RadioButton()
.build({
  owner : optionsPanel,
  manager : optionsPanel.radio,
  parent : optionsPanel.container,
  x : 60,
  y : 80,
  caption : {
    position : "right",                  // defaults to "left"
    y : 12,
    textNode : "Option 1"
  },
  isDefault : true
});

radioButton2 = new pergola.RadioButton()
.build({
  owner : optionsPanel,
  manager : optionsPanel.radio,
  parent : optionsPanel.container,
  x : 60,
  y : 108,
  caption : {
    position : "right",
    y : 12,
    textNode : "Option 2"
  }
});




checkBox1 = new pergola.CheckBox()
.build({
  owner : optionsPanel,
  manager : optionsPanel.check,
  parent : optionsPanel.container,
  x : 60,
  y : 140,
  checked : false,
  caption : {
    position : "right",
    y : 12,
    textNode : "Option 3"
  }
});

checkBox2 = new pergola.CheckBox()
.build({
  owner : optionsPanel,
  manager : optionsPanel.check,
  parent : optionsPanel.container,
  x : 60,
  y : 168,
  checked : false,
  caption : {
    position : "right",
    y : 12,
    textNode : "Option 4"
  }
});

