
//=============================   pergola examples - sliders   ==========================




var g = $C({element : "g", id : "Sliders", transform : "translate(20 40)", appendTo : pergola.user});


mySlider1 = new pergola.Slider("My Slider 1")
.buildH({
  parent : g
});


mySlider2 = new pergola.Slider("My Slider 2")
.buildH({
  parent : g,
  y : 100,
  extent : 150,
  shape : "square",
  type : "discrete",
  numberOfSteps : 6,
  valueTip : true,
  stroke : "sandybrown",
  slotSize : 2,
  slotStroke : "#8080A0"
});


mySlider3 = new pergola.Slider("My Slider 3")
.buildH({
  parent : g,
  y : 200,
  size : 22,
  extent : 180,
  initial : "start",
  slotSize : 2
});


mySlider4 = new pergola.Slider("My Slider 4")
.buildH({
  parent : g,
  y : 300,
  size : 12,
  initial : "end",
  valueTip : true
});


mySlider8 = new pergola.Slider("My Slider 8")
.buildV({
  parent : g,
  x : 280,
  extent : 100,
  size : 11,
  fill : "white",
  stroke : "lightblue",
  trackSize : 11,
  trackFill : "white",
  trackStroke : "lightblue",
  trackOpacity : 1,
  initial : "end",
  slotSize : 0
});


mySlider5 = new pergola.Slider("My Slider 5")
.buildV({
  parent : g,
  valueTip : true,
  x : 380,
  stroke : "slategray",
  strokeWidth : 2,
  slotStroke : "slategray",
  slotSize : 2
});


mySlider6 = new pergola.Slider("My Slider 6")
.buildV({
  parent : g,
  x : 480,
  extent : 180,
  shape : "square",
  size : 11,
  valueTip : true,
  initial : "start",
  type : "discrete",
  numberOfSteps : 10
});


// a target object for the next slider below.
var circle = $C({element : "circle", cx : 640, cy : 109, r : 16, fill : "#00FFFF", stroke : "gray", "stroke-width" : 2, opacity : .25, appendTo : g});

mySlider7 = new pergola.Slider("My Slider 7")
.buildV({
  parent : g,
  x : 580,
  extent : 220,
  shape : "square",
  size : 20,
  slotSize : 2,
  slotStroke : "#909090",
  initial : "start",
  target : circle,
  fn : "myFunction",
  valueTip : true,
  quickTip : {
    tip : "rgb(000, 255, 255)",
    delay : 0,
    x : 688,
    y : 140
  }
});


mySlider7.myFunction = function (evt) {
  var ratio = 255 / this.extent;
  var value = parseInt(this.pos * ratio);
  var rgb = "rgb(" + value + ", " + (255 - value) + ", 255)";
  this.target.setAttributeNS(null, "fill", rgb);
  this.quickTip.update([rgb]);
}