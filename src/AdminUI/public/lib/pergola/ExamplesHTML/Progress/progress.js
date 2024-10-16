
//=============================   Pergola examples - progress bar   ==========================


var progressBar1 = new pergola.Progress()
.build({
  owner : pergola,
  x : 100,
  y : 50
});

var progressBar2 = new pergola.Progress()
.build({
  owner : pergola,
  x : 100,
  y : 200,
  width : 240,
  height : 7,
  fill : "none",
  stroke : "gray",
  statusFill : "#00E000",
  extra : {rx : 3.5}
});




var button1 = new pergola.DialogButton()
.build({
  x : 100,
  y : 100,
  text : "Start",
  ev : "click",
  fn : "startProgress",
  target : progressBar1,
  startProgress : function (evt) {
    this.unregisterEvents(this.button, ["click"]);
    this.target.start();
    if (this.timer) this.timer.initialize();
    else {
      this.timer = pergola.Timer()
      .initialize({
        handle : this,
        callback : this.progress,
        frequence : this.frequence || 20
      });
    }
  },
  progress : function (timer) {
    var o = this.target;
    if (o.advance == o.extent) {
      this.timer.clear();
      o.stop();
      this.registerEvents(this.button, ["click"]);
      return;
    }
    o.status.setAttributeNS(null, "width", ++o.advance);
  }
});


var button2 = new pergola.DialogButton()
.build({
  x : 100,
  y : 250,
  text : "Start",
  ev : "click",
  fn : "startProgress",
  target : progressBar2,
  progress  : button1.progress,
  startProgress : button1.startProgress,
  frequence : 15
});



