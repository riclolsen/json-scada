
//=============================   Pergola examples - Tool Buttons   ==========================




var toolBar = new pergola.ToolBar()
.build({
  x : 80,
  y : 80,
  width : 420
});


var buttonsGroup = $C({element : "g", transform : "translate(40 3)", appendTo : toolBar.container});


var closeButton = new pergola.CommandButton("close");
closeButton.build({
  parent : buttonsGroup,
  y : 4,
  symbol : {
    symbol : pergola.symbols.winClose,
    x : 5,
    y : 4
  }
});


var emptyTool = new pergola.ToolButton()
.build({
  parent : buttonsGroup,
  x : 60,
  width : toolBar.height,
  height : toolBar.height - 6,
  extra : {
    rx : 7
  }
});


var handTool = new pergola.ToolButton()
.build({
  parent : buttonsGroup,
  x : 140,
  width : toolBar.height,
  height : toolBar.height - 6,
  extra : {
    rx : 7
  },
  symbol : {
    symbol : pergola.symbols.hand,
    x : 8,
    y : 7
  },
  quickTip : "handTool"
});


var barrelToolButton = new pergola.ToolButton()
.build({
  parent : buttonsGroup,
  x : 220,
  width : toolBar.height - 4,
  height : toolBar.height - 6,
  stroke : "peru",
  maskFill : "#E8E8E8",
  extra : {
    rx : 4,
    ry : 11
  },
  quickTip : {tip : "BYOB"}
});


var colorPickerToolButton = new pergola.ToolButton()
.build({
  parent : buttonsGroup,
  x : 296,
  width : toolBar.height,
  height : toolBar.height - 6,
  maskFill : "#F0F0F0",
  extra : {
    rx : 7
  },
  stroke : pergola.color.shade([104.761, 99.009, 91.847], -15),
  symbol : {
    symbol : pergola.path + "lib/symbols/spectrum.png",
    width : 16,
    height : 16,
    x : 6.5,
    y : 4
  }
});