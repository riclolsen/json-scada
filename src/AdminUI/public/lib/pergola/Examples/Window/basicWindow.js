
//=============================   Pergola examples - window basic   ==========================



var myWindow = new pergola.Window()
.build({
//  hasCommands : false,
//  hasToolBar : false,
//  hasZoomAndPan : false,
//  hasStatus : false,
  contextMenus : {
  	status : {
		  changeToRed : {
		    string : "Change To Red",
		    target : this,
		    fn : function (evt) {alert(this.name)}
		  },
		  changetoBlue : {
		    string : "Change To Blue",
		    fn : function (evt) {alert("")}
		  },
		  rotate : {
		    string : "Rotate 45°",
		    fn : function (evt) {alert("")}
		  },
		  remove : {
		  	separator : new pergola.Separator(),
		    string : "Delete",
		    fn : function () {p.parentNode.removeChild(p);}
		  }
		},
		topBar : {
		  changeToRed : {
		    string : "Change To Red",
		    target : this,
		    fn : function (evt) {alert(this.name)}
		  },
		  changetoBlue : {
		    string : "Change To Blue",
		    fn : function (evt) {alert("")}
		  },
		  rotate : {
		    string : "Rotate 45°",
		    fn : function (evt) {alert("")}
		  },
		  remove : {
		  	separator : new pergola.Separator(),
		    string : "Delete",
		    fn : function () {p.parentNode.removeChild(p);}
		  }
		},
		tab : {
		  changeToRed : {
		    string : "Change To Red",
		    target : this,
		    fn : function (evt) {alert(this.name)}
		  },
		  changetoBlue : {
		    string : "Change To Blue",
		    fn : function (evt) {alert("")}
		  },
		  rotate : {
		    string : "Rotate 45°",
		    fn : function (evt) {alert("")}
		  },
		  remove : {
		  	separator : new pergola.Separator(),
		    string : "Delete",
		    fn : function () {p.parentNode.removeChild(p);}
		  }
		},
		toolBar : {
		  changeToRed : {
		    string : "Change To Red",
		    target : this,
		    fn : function (evt) {alert(this.name)}
		  },
		  changetoBlue : {
		    string : "Change To Blue",
		    fn : function (evt) {alert("")}
		  },
		  rotate : {
		    string : "Rotate 45°",
		    fn : function (evt) {alert("")}
		  },
		  remove : {
		  	separator : new pergola.Separator(),
		    string : "Delete",
		    fn : function () {p.parentNode.removeChild(p);}
		  }
		},
		doc : {
		  changeToRed : {
		    string : "Change To Red",
		    target : this,
		    fn : function (evt) {alert(this.name)}
		  },
		  changetoBlue : {
		    string : "Change To Blue",
		    fn : function (evt) {alert("")}
		  },
		  rotate : {
		    string : "Rotate 45°",
		    fn : function (evt) {alert("")}
		  },
		  remove : {
		  	separator : new pergola.Separator(),
		    string : "Delete",
		    fn : function () {p.parentNode.removeChild(p);}
		  }
		}
	}
});

/*
myWindow.status.contextMenuItems = {
	changeToRed : {
    string : "Change To Red",
    target : this,
    fn : function (evt) {alert(this.name)}
  },
  changetoBlue : {
    string : "Change To Blue",
    fn : function (evt) {alert("")}
  },
  rotate : {
    string : "Rotate 45°",
    fn : function (evt) {alert("")}
  },
  remove : {
    separator : new pergola.Separator(),
		string : "Delete",
    fn : function () {p.parentNode.removeChild(p);}
  }
};
*/
