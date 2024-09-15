
//=============================   Pergola examples - custom context menu   ==========================


/*
 * Right click on shape shows its custom context menu.
 * Right click anywhere else on the interface shows the browser's context menu.  
*/


var p = $C({element : "path", d : "M100,100h68v68h-68", fill : "#585858", appendTo : pergola.user});
p.addEventListener("mousedown", handler, false);

p.contextMenuItems = {
  changeToRed : {
    string : "Change To Red",
    fn : function () {p.setAttributeNS(null, "fill", "red");}
  },
  changetoBlue : {
    string : "Change To Blue",
    fn : function () {p.setAttributeNS(null, "fill", "blue");}
  },
  rotate : {
    string : "Rotate 45°",
    fn : function () {p.setAttributeNS(null, "transform", "rotate(45, 130, 130)");}    
  },
  remove : {
  	separator : new pergola.Separator(),  
    string : "Delete",
    fn : function () {p.parentNode.removeChild(p);}
  }
};

function handler (evt) {
  if (evt.button == 2) {
    pergola.contextmenuManager.activate(evt, handler, evt.target.contextMenuItems);
    return;
  }
}





