#!/usr/bin/env python

import sys
import json
import inkex
from inkex.elements import TextElement, Group, Rectangle
import tkinter as tk
from tkinter import ttk,font,colorchooser

from inkex.utils import errormsg

root = tk.Tk()
root.title("SCADA")
root.minsize(540,500)

app_width = 540
app_height = 600

screen_width = root.winfo_screenwidth()
screen_height = root.winfo_screenheight()

x = screen_width - app_width
y = (screen_height/2) - (app_height/2)

root.geometry(f'{app_width}x{app_height}+{int(x)-15}+{int(y)}')

object_id_label = tk.Label(root, text = "ID: ")
object_id_entry = tk.Entry(root)
object_id_entry.focus_set()

frame = tk.LabelFrame(root, text = "SCADA", padx = 5, pady = 5)
frame.grid(row =1, columnspan = 2,  sticky ='NSWE')
default_font = font.nametofont("TkDefaultFont")
#Creating Tab Control
tabControl = ttk.Notebook(frame)
#Creating the tabs
#Adding the tab & Packing the tab control to make the tabs visible

#region BAR GUI tab configuration setting
bartab = ttk.Frame(tabControl)      #tab for BAR
tabControl.add(bartab, text='Bar')
tabControl.pack(expand=1, fill="both")

bartag_var = tk.StringVar()            # for BAR info
barmax_var = tk.StringVar()
barmin_var = tk.StringVar()
barmin_var.set('0')
barmax_var.set('1')

bartag_label = tk.Label(bartab, text = "Tag* ", anchor="w")
barmin_label = tk.Label(bartab, text = "Min " , anchor="w")
barmax_label = tk.Label(bartab, text = "Max " , anchor="w")

bartag_entry = tk.Entry(bartab, textvariable = bartag_var)
barmin_entry = tk.Entry(bartab, textvariable = barmin_var)
barmax_entry = tk.Entry(bartab, textvariable = barmax_var)
#endregion

#region FACEPLATE GUI tab configuration setting
faceplatetab = ttk.Frame(tabControl)      #tab for FACEPLATE
tabControl.add(faceplatetab, text='Faceplate')
tabControl.pack(expand=1, fill="both")

faceplate_frame = tk.Frame(faceplatetab)        # for Faceplate info
faceplate_frame.grid(row = 0, columnspan =2 ,sticky='NEWS')

faceplate_tree = ttk.Treeview(faceplate_frame)
faceplate_tree['columns'] = ("Variable","=","Value")
global faceplate_count
faceplate_count = 0

#Faceplate LABELs
faceplate_variable_label = tk.Label(faceplate_frame, text = 'Variable')
faceplate_value_label = tk.Label(faceplate_frame, text = 'Value')

#Faceplate ENTERies
faceplate_variable_entry = tk.Entry(faceplate_frame)
faceplate_value_entry = tk.Entry(faceplate_frame)
#Format columns
faceplate_tree.column("#0",width=0, stretch=tk.NO)
faceplate_tree.column("Variable",minwidth=70, width=70, anchor=tk.CENTER,stretch=tk.NO)
faceplate_tree.column("=",minwidth=25,width=25,anchor=tk.CENTER,stretch=tk.NO)
faceplate_tree.column("Value",minwidth=80,width=80, anchor=tk.W)

faceplate_tree.heading("#0",text="",anchor= tk.W)
faceplate_tree.heading("Variable",text="Variable",anchor=tk.CENTER)
faceplate_tree.heading("=",text="=",anchor=tk.CENTER)
faceplate_tree.heading("Value",text="Value",anchor=tk.CENTER)

faceplate_tree.grid(row = 2 , columnspan =2 ,sticky = 'NEWS')
#endregion

#region COLOR GUI tab configuration setting
colortab = ttk.Frame(tabControl)      #tab for ABOUT
tabControl.add(colortab, text='Color')
tabControl.pack(expand=1, fill="both")

color_frame = tk.Frame(colortab)            # for COLOR info
color_frame.grid(row = 0, columnspan =2 ,sticky='NEWS')

color_variable_label = tk.Label(color_frame, text = 'Tag')
color_limit_label = tk.Label(color_frame, text = 'Limit')
color_name_code_label = tk.Label(color_frame, text = 'Color Name/Code')

color_variable_entry = tk.Entry(color_frame)
color_limit_entry = tk.Entry(color_frame)
color_name_code_entry = tk.Entry(color_frame)

color_tree = ttk.Treeview(color_frame)
color_tree['columns'] = ("Tag","Limit","Color Name/Code")
global color_count
color_count = 0
global colorpicker_flag
colorpicker_flag = 0

#Format columns
color_tree.column("#0", width=0, stretch=tk.NO)
color_tree.column("Tag", minwidth=120, width=120, anchor=tk.W)
color_tree.column("Limit", minwidth=120, width=120, anchor=tk.W)
color_tree.column("Color Name/Code", minwidth=90, width=90, anchor=tk.W)

color_tree.heading("#0", text="", anchor=tk.W)
color_tree.heading("Tag", text="Tag", anchor=tk.CENTER)
color_tree.heading("Limit", text="Limit", anchor=tk.CENTER)
color_tree.heading("Color Name/Code", text="Color Name/Code", anchor=tk.CENTER)

color_tree.grid(row=2, rowspan = 4, columnspan=3, sticky='NEWS', pady = 5)
# endregion

#region OPACITY GUI tab configuration setting
opactab = ttk.Frame(tabControl)     #tab for OPACITY
tabControl.add(opactab, text='Opacity')
tabControl.pack(expand=1, fill="both")

opactag_var = tk.StringVar()            # for OPACITY info
opacmax_var = tk.StringVar()
opacmin_var = tk.StringVar()
opacmin_var.set('0')
opacmax_var.set('1')

opactag_label = tk.Label(opactab, text = "Tag* ", anchor="w")
opacmin_label = tk.Label(opactab, text = "Min " , anchor="w")
opacmax_label = tk.Label(opactab, text = "Max " , anchor="w")

opactag_entry = tk.Entry(opactab, textvariable = opactag_var)
opacmin_entry = tk.Entry(opactab, textvariable = opacmin_var)
opacmax_entry = tk.Entry(opactab, textvariable = opacmax_var)
#endregion

#region OPEN GUI tab configuration setting
opentab = ttk.Frame(tabControl)      #tab for GET
tabControl.add(opentab, text='Open')
tabControl.pack(expand=1, fill="both")

opensource_var = tk.StringVar()         # for OPEN info
opensource_type_var = tk.StringVar()
opendest_type_var = tk.StringVar()
open_xpos_var = tk.StringVar()
open_ypos_var = tk.StringVar()
open_width_var = tk.StringVar()
open_height_var = tk.StringVar()
open_xpos_var.set('100')
open_ypos_var.set('100')
open_width_var.set('500')
open_height_var.set('400')

opensource_label = tk.Label(opentab, text = "Source* ",anchor="w")
opensource_type_label = tk.Label(opentab, text = "Source Type" ,anchor="w")
opendest_type_label = tk.Label(opentab, text = "Dest. Type" ,anchor="w")
open_xpos_label = tk.Label(opentab, text = "X-position" ,anchor="w")
open_ypos_label = tk.Label(opentab, text = "Y-position" ,anchor="w")
openwidth_label = tk.Label(opentab, text = "Width" ,anchor="w")
openheight_label = tk.Label(opentab, text = "Height" ,anchor="w")

opensource_entry = tk.Entry(opentab, textvariable = opensource_var)
open_xpos_entry = tk.Entry(opentab, textvariable = open_xpos_var, state='disable')
open_ypos_entry = tk.Entry(opentab, textvariable = open_ypos_var, state='disable')
openwidth_entry = tk.Entry(opentab, textvariable = open_width_var, state='disable')
openheight_entry = tk.Entry(opentab, textvariable = open_height_var, state='disable')

opensource_type_combo = ttk.Combobox(opentab, textvariable = opensource_type_var, state = "readonly")
opensource_type_combo['values'] = ('URL','Tag')
opensource_type_combo.current(0)

opendest_type_combo = ttk.Combobox(opentab, textvariable = opendest_type_var , state = "readonly")
opendest_type_combo['values'] = ('Current Window','New exclusive window', 'New shared window')
opendest_type_combo.current(0)
#endregion

#region POP GUI tab configuration setting
popuptab = ttk.Frame(tabControl)     #tab for POPUP
tabControl.add(popuptab, text='Popup')
tabControl.pack(expand=1, fill="both")

popupsrc_var = tk.StringVar()           # for POPUP info
popup_xpos_var = tk.StringVar()
popup_ypos_var = tk.StringVar()
popupwidth_var = tk.StringVar()
popupheight_var = tk.StringVar()

popupsrc_label = tk.Label(popuptab, text = "Source* ", anchor="w")
popup_xpos_label = tk.Label(popuptab, text = "X-position " , anchor="w")
popup_ypos_label = tk.Label(popuptab, text = "Y-position " , anchor="w")
popupwidth_label = tk.Label(popuptab, text = "Width " , anchor="w")
popupheight_label = tk.Label(popuptab, text = "Height " , anchor="w")

popupsrc_entry = tk.Entry(popuptab, textvariable = popupsrc_var)
popup_xpos_entry = tk.Entry(popuptab, textvariable = popup_xpos_var)
popup_ypos_entry = tk.Entry(popuptab, textvariable = popup_ypos_var)
popupwidth_entry = tk.Entry(popuptab, textvariable = popupwidth_var)
popupheight_entry = tk.Entry(popuptab, textvariable = popupheight_var)

popup_xpos_var.set("100")
popup_ypos_var.set("100")
popupwidth_var.set("500")
popupheight_var.set("400")
#endregion

#region ROTATE GUI tab configuration setting
rotatetab = ttk.Frame(tabControl)   #tab for ROTATE
tabControl.add(rotatetab, text='Rotate')
tabControl.pack(expand=1, fill="both")

rotatetag_var = tk.StringVar()          # for ROTATE info
rotatemax_var = tk.StringVar()
rotatemin_var = tk.StringVar()
rotatemax_var.set('100')
rotatemin_var.set('0')

rotatetag_label = tk.Label(rotatetab, text = "Tag* ", anchor="w")
rotatemin_label = tk.Label(rotatetab, text = "Min " , anchor="w")
rotatemax_label = tk.Label(rotatetab, text = "Max " , anchor="w")

rotatetag_entry = tk.Entry(rotatetab, textvariable = rotatetag_var)
rotatemin_entry = tk.Entry(rotatetab, textvariable = rotatemin_var)
rotatemax_entry = tk.Entry(rotatetab, textvariable = rotatemax_var)
#endregion

#region SCRIPT GUI tab configuration setting
scripttab = ttk.Frame(tabControl)      #tab for SCRIPT
tabControl.add(scripttab, text='Script')
tabControl.pack(expand=1, fill="both")

script_tree = ttk.Treeview(scripttab)
script_tree['columns'] = ("Event")

#Format columns
script_tree.column("#0",width=0, stretch=tk.NO)
script_tree.column("Event",minwidth=80, width=80, anchor=tk.W,stretch=tk.NO)

script_tree.heading("#0",text="",anchor= tk.W)
script_tree.heading("Event",text="Event",anchor=tk.CENTER)

script_event = ["mouseup","mousedown","mouseover","mouseout","mousemove","keydown","exec_once","exec_on_update","vega-lite","vega","vega-json","vega4","vega4-json"]

scripttexts = []
script_count = 0
for script_record in script_event:
    script_tree.insert(parent = '', index='end', iid=script_count,text="",values=(script_record))
    script_count += 1

script_tree.selection_set(0)
script_tree.grid(row = 0 , column=0, sticky='NSW')

scriptframe = tk.Frame(scripttab, padx = 5 ,pady = 5)
scriptframe.grid(row =0, column = 1,  sticky ='NSWE')
scriptframe.option_add("*Font", default_font)

keydownframe = tk.Frame(scripttab, padx = 5 ,pady = 5)
keydownframe.option_add("*Font", default_font)

s = 0
for s in range(13):
    script_text = tk.Text(scriptframe, borderwidth=2, width =0, height=0)
    scripttexts.append(script_text)
    scripttexts[s].delete('1.0',tk.END)
#keydwon
#Put Ctrl, ALt, Shift , Key
keydownCtrl_var = tk.IntVar()
keydownAlt_var = tk.IntVar()
keydownShift_var = tk.IntVar()
keydownKey_var = tk.StringVar()
keydownKEYS = []

keydownCtrl_checkbox = tk.Checkbutton(keydownframe, text = "Ctrl", variable = keydownCtrl_var)
keydownAlt_checkbox = tk.Checkbutton(keydownframe, text = "Alt", variable = keydownAlt_var)
keydownShift_checkbox = tk.Checkbutton(keydownframe,text='Shift', variable = keydownShift_var)
keydownKey_label = tk.Label(keydownframe,text='Key: ')
keydownKey_entry = tk.Entry(keydownframe, textvariable=keydownKey_var)
keydown_text = tk.Text(keydownframe, borderwidth=2, width =0, height=0)
#endregion

#region GET GUI tab configuration setting
gettab = ttk.Frame(tabControl)      #tab for GET
tabControl.add(gettab, text='Get')
tabControl.pack(expand=1, fill="both")

gettag_var = tk.StringVar()             # for GET info
getalign_var = tk.StringVar()
gettype_var = tk.StringVar()

gettag_label = tk.Label(gettab, text = "Tag* " ,anchor="w")
getalign_label = tk.Label(gettab, text = "Alignment ",anchor="w")
gettype_label = tk.Label(gettab, text = "Type ",anchor="w")

gettag_entry = tk.Entry(gettab, textvariable = gettag_var)

getalign_combo = ttk.Combobox(gettab, textvariable = getalign_var, state = 'readonly')
getalign_combo['values'] = ('Right','Left')
getalign_combo.current(0)

gettype_combo = ttk.Combobox(gettab, textvariable = gettype_var, state = 'readonly')
gettype_combo['values'] = ('Good','Live')
gettype_combo.current(0)
#endregion

#region SET GUI tab configuration setting
settab = ttk.Frame(tabControl)      #tab for SET
tabControl.add(settab, text='Set')
tabControl.pack(expand=1, fill="both")

settag_var = tk.StringVar()         # for SET info
setalign_var = tk.StringVar()
setprompt_var = tk.StringVar()
setsource_var =tk.StringVar()
settype_var = tk.StringVar()

settag_label = tk.Label(settab, text = "Tag* " ,anchor="w")
setalign_label = tk.Label(settab, text = "Alignment ",anchor="w")
setprompt_label = tk.Label(settab, text = "Prompt " ,anchor="w")
setsource_label = tk.Label(settab, text = "Source " ,anchor="w")
settype_label = tk.Label(settab, text = "Type ",anchor="w")

settag_entry = tk.Entry(settab, textvariable = settag_var)

setalign_combo = ttk.Combobox(settab, textvariable = setalign_var, state = 'readonly')
setalign_combo['values'] = ('Right','Left')
setalign_combo.current(0)

setprompt_entry = tk.Entry(settab, textvariable = setprompt_var)

setsource_entry = tk.Entry(settab, textvariable = setsource_var)

settype_combo = ttk.Combobox(settab, textvariable = settype_var, state = 'readonly')
settype_combo['values'] = ('Data','Variable')
settype_combo.current(0)
#endregion

#region SLIDER GUI tab configuration setting
slidertab = ttk.Frame(tabControl)   #tab for SLIDER
tabControl.add(slidertab, text='Slider')
tabControl.pack(expand=1, fill="both")

slidertag_var = tk.StringVar()          # for SLIDER info
slidermax_var = tk.StringVar()
slidermin_var = tk.StringVar()
sliderread_var = tk.IntVar()
slidermax_var.set('100')
slidermin_var.set('0')

slidertag_label = tk.Label(slidertab, text = "Tag* " ,anchor="w")
slidermin_label = tk.Label(slidertab, text = "Min " ,anchor="w")
slidermax_label = tk.Label(slidertab, text = "Max " ,anchor="w")

slidertag_entry = tk.Entry(slidertab, textvariable = slidertag_var)
slidermin_entry = tk.Entry(slidertab, textvariable = slidermin_var)
slidermax_entry = tk.Entry(slidertab, textvariable = slidermax_var)

sliderread_checkbox = tk.Checkbutton(slidertab, text = "Read Only",variable = sliderread_var)
#endregion

#region TOOLTIPS GUI tab configuration setting
tooltipstab = ttk.Frame(tabControl)     #tab for TOOLTIPS
tabControl.add(tooltipstab, text='Tooltips')
tabControl.pack(expand=1, fill="both")

tooltipsentries = []                    # for TOOLTIPS info
tooltipssize_var = tk.StringVar()
tooltipsstyle_var = tk.StringVar()

for x in range(5):
    tooltipsline_label = tk.Label(tooltipstab, text = "Line {}".format(x + 1), anchor="w" , padx=3 , pady = 3)
    tooltipsline_entry = tk.Entry(tooltipstab)
    tooltipsentries.append(tooltipsline_entry)
    tooltipsline_label.grid(row = x, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    tooltipsentries[x].grid(row = x, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)

tooltipssize_label = tk.Label(tooltipstab, text = "Size " , anchor="w" , padx=3 , pady = 3)
tooltipsstyle_label = tk.Label(tooltipstab, text = "Style " , anchor="w" , padx=3 , pady = 3)

tooltipssize_entry = tk.Entry(tooltipstab, textvariable = tooltipssize_var)
tooltipsstyle_entry = tk.Entry(tooltipstab, textvariable = tooltipsstyle_var)
#endregion

#region ZOOM GUI tab configuration setting
zoomtab = ttk.Frame(tabControl)      #tab for ZOOM
tabControl.add(zoomtab, text='Zoom')
tabControl.pack(expand=1, fill="both")

zoomalign_var = tk.StringVar()            # for ZOOM info
zoomtag_var = tk.StringVar()

zoomalign_label = tk.Label(zoomtab, text = "Alignment* ",anchor="w")
zoomtag_label = tk.Label(zoomtab, text = "Tag " ,anchor="w")

zoomtag_entry = tk.Entry(zoomtab, textvariable = zoomtag_var)

zoomalign_combo = ttk.Combobox(zoomtab, textvariable = zoomalign_var ,state='readonly')
zoomalign_combo['values'] = ('None','Center','Left','Right','Top','Bottom')
zoomalign_combo.current(0)
#endregion

#region TEXT GUI tab configuration setting
texttab = ttk.Frame(tabControl)      #tab for TEXT
tabControl.add(texttab, text='Text')
tabControl.pack(expand=1, fill="both")

text_TagValues = []                         # for TEXT info
text_TagTexts= []
texttag_var = tk.StringVar()

texttag_label = tk.Label(texttab, text = "Tag* " ,anchor="w")
texttag_entry = tk.Entry(texttab, textvariable = texttag_var)

insideframe = tk.Frame(texttab, padx = 5 ,pady = 5 ,relief='sunken', borderwidth=1)
insideframe.grid(row =1, columnspan = 2,  sticky ='NSWE')
texttag_value_label = tk.Label(insideframe, text = "Tag Value")
texttag_text_label = tk.Label(insideframe, text = "Tag Text")
insideframe.columnconfigure(2, weight =1)

for t in range (1,9):
    textTagValueEntry = tk.Entry(insideframe)
    textTagTextEntry = tk.Entry(insideframe)
    text_TagValues.append(textTagValueEntry)
    text_TagTexts.append(textTagTextEntry)
    textTagValueEntry.grid(row =t, column = 0, padx = 4 , pady = 4)
    texttext = tk.Label(insideframe, text = '=').grid(row = t,column = 1)
    textTagTextEntry.grid(row = t, column = 2, padx = 4 , pady = 4,sticky = 'NSWE')
#endregion

#region ABOUT GUI tab configuration setting
abouttab = ttk.Frame(tabControl)      #tab for ABOUT
tabControl.add(abouttab, text='About')
tabControl.pack(expand=1, fill="both")

abouttext = tk.StringVar()          # for ABOUT info

aboutframe = tk.Frame(abouttab, padx = 5 ,pady = 5 ,bg="white")
aboutframe.grid(row =0, columnspan = 2,  sticky ='NSWE')

aboutframe.option_add("*Font", default_font)
abouttext = tk.Text(aboutframe, wrap=tk.WORD, borderwidth=2)

abouttext.insert(tk.END,"\nSCADA Animation GUI Editor eXtension. Based on ECAVA/IntegraXor SCADA system.")
#endregion

animations = [  'ttr":"bar"','ttr":"clone"','ttr":"color"','ttr":"opac"',\
                'ttr":"open"','ttr":"popup"','ttr":"rotate"','ttr":"script"',\
                'ttr":"get"','ttr":"set"','ttr":"slider"','ttr":"tooltips"',\
                'ttr":"zoom"','ttr":"text"']

animalist = []

class SAGEX(inkex.EffectExtension):
    def effect(self):
        if len(self.options.ids) == 0:
            # object_id_label.configure(state=tk.DISABLED)
            # object_id_entry.configure(state=tk.DISABLED)
            message = ' "Usage: Please select the desired element(s) then open SAGEX for SCADA animation".'
            object_id_entry.delete(0,'end')
            object_id_entry.insert(0,message)
            object_id_entry.configure(fg='red')

            for t in range(len(tabControl.tabs())):
                tabControl.tab(t,state=tk.DISABLED)

        for node in self.svg.selected:
            attr_id = node.attrib['id']
            object_id_entry.delete(0,'end')
            object_id_entry.insert(0,attr_id)

            if not isinstance(node, Rectangle):
                tabControl.hide(bartab)
            if not isinstance(node, TextElement):
                tabControl.hide(gettab)
                tabControl.hide(texttab)
            if isinstance(node, TextElement):
                tabControl.hide(zoomtab)
                tabControl.hide(tooltipstab)
            if isinstance(node, Group):
                tabControl.hide(colortab)
            

            current_attr = node.get(inkex.addNS('label','inkscape'))
            if current_attr != None and current_attr.find('ttr":') != -1:
                find_firstattr = current_attr.find('ttr":')
                first_attr = current_attr[find_firstattr:]
                find_firstattrend = first_attr.find('","')
                first_attr = first_attr[:find_firstattrend+1]
                s_index = animations.index(first_attr)
                tabControl.select(s_index)

def jsonitize( usrJson):
  try:
    listjson = json.loads( usrJson)
  except ValueError as e:
    return ""
  return listjson

class DisplayBar(inkex.EffectExtension):
    def effect(self):
        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))

            if current_attr != None:
                bar_location = current_attr.find(':"bar"')
                if bar_location != -1:
                    tabControl.tab(0,text = 'Bar*')
                    sliced_bar = current_attr[bar_location-7:]
                    sliced_bar_end = sliced_bar.find(',{')

                    if sliced_bar_end >= 0:
                        sliced_bar = sliced_bar[:sliced_bar_end]

                    barjson = jsonitize(sliced_bar)

                    barmax_entry.delete(0,'end')
                    barmax_entry.insert(0,barjson["max"])

                    barmin_entry.delete(0,'end')
                    barmin_entry.insert(0,barjson["min"])

                    bartag_entry.delete(0,'end')
                    bartag_entry.insert(0,barjson["tag"])

class DisplayFaceplate(inkex.EffectExtension):
    def effect(self):
        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))

            if current_attr != None and current_attr.find(':"clone"') != -1:
                tabControl.tab(1,text = 'Faceplate*')
                faceplate_location = current_attr.find(':"clone"')
                sliced_faceplate = current_attr[faceplate_location-7:]

                find_faceplate_end = sliced_faceplate.find(']}')
                sliced_faceplate = sliced_faceplate[faceplate_location - 7:find_faceplate_end+2]
                # inkex.errormsg(("sliced",sliced_faceplate))
                faceplatejson = jsonitize(sliced_faceplate)
                mapjson = faceplatejson['map']

                maplist_splitequal = []

                for m in range(len(mapjson)):

                    maplist_splitequal = mapjson[m].split("=")    #split the equal for each list
                    faceplate_tree.insert(parent='',index="end",text="",\
                                        values=(maplist_splitequal[0][1:],"=",maplist_splitequal[1]))

class DisplayColor(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))

            if current_attr != None and current_attr.find(':"color"') != -1:
                tabControl.tab(2,text = 'Color*')
                color_location = current_attr.find(':"color"')

                sliced_color = current_attr[color_location:]

                find_list = sliced_color.find('"list":')
                find_color_end = sliced_color.find('"}]}')

                get_list = sliced_color[find_list + 7 : find_color_end + 3]

                # inkex.errormsg(("LIST: ",get_list))
                listjson = jsonitize(get_list)

                for i in range(len(listjson)):
                    color_tree.insert(parent='',index='end', iid=i,text="",\
                        values=(listjson[i].get("tag"),listjson[i].get("data"),listjson[i].get("param")))

class DisplayOpacity(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))
            if current_attr != None:
                opac_location = current_attr.find(':"opac"')

                # inkex.errormsg(("END loc",opac_end_location))
                # inkex.errormsg(("Before",current_attr))
                if opac_location != -1:
                    tabControl.tab(3,text = 'Opacity*')
                    sliced_opac = current_attr[opac_location-7:]
                    sliced_opac_end = sliced_opac.find(',{')

                    if sliced_opac_end >= 0:
                        sliced_opac = sliced_opac[:sliced_opac_end]

                    # inkex.errormsg(("After",sliced_opac))

                    opacjson = jsonitize(sliced_opac)

                    get_min = opacjson["min"]
                    get_max = opacjson["max"]
                    get_tag = opacjson["tag"]

                    opacmax_entry.delete(0,'end')
                    opacmax_entry.insert(0,get_max)

                    opacmin_entry.delete(0,'end')
                    opacmin_entry.insert(0,get_min)

                    opactag_entry.delete(0,'end')
                    opactag_entry.insert(0,get_tag)

                    # inkex.errormsg(("JSON min",get_min))
                    # inkex.errormsg(("JSON min",get_max))
                    # inkex.errormsg(("JSON min",get_tag))

class DisplayOpen(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            current_attr = node.get(inkex.addNS('label','inkscape'))

            if current_attr != None and current_attr.find(':"open"') != -1:
                tabControl.tab(4,text = 'Open*')
                open_location = current_attr.find(':"open"')

                sliced_open = current_attr[open_location-7:]
                # inkex.errormsg(("so",sliced_open))
                sliced_open_end = sliced_open.find(',{')

                if sliced_open_end >= 0:
                    sliced_open = sliced_open[:sliced_open_end]
                # inkex.errormsg(sliced_open)

                openjson = jsonitize(sliced_open)

                opensource_entry.delete(0,'end')
                opensource_entry.insert(0,openjson['src'])

                if openjson['istag'] == 1:
                    opensource_type_combo.current(1)
                elif openjson['istag'] == 0:
                    opensource_type_combo.current(0)

                if openjson['type'] == "_self":

                    opendest_type_combo.current(0)
                    open_xpos_entry.configure(state="disable")
                    open_ypos_entry.configure(state="disable")
                    openwidth_entry.configure(state="disable")
                    openheight_entry.configure(state="disable")

                elif openjson['type'] == "_blank":
                    opendest_type_combo.current(1)
                    open_xpos_entry.configure(state="normal")
                    open_ypos_entry.configure(state="normal")
                    openwidth_entry.configure(state="normal")
                    openheight_entry.configure(state="normal")

                elif openjson['type'] == "_shared":
                    opendest_type_combo.current(2)
                    open_xpos_entry.configure(state="normal")
                    open_ypos_entry.configure(state="normal")
                    openwidth_entry.configure(state="normal")
                    openheight_entry.configure(state="normal")

                open_xpos_entry.delete(0,'end')
                open_xpos_entry.insert(0,openjson['x'])

                open_ypos_entry.delete(0,'end')
                open_ypos_entry.insert(0,openjson['y'])

                openwidth_entry.delete(0,'end')
                openwidth_entry.insert(0,openjson['width'])

                openheight_entry.delete(0,'end')
                openheight_entry.insert(0,openjson['height'])

class DisplayPopup(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))

            if current_attr != None:
                popup_location = current_attr.find(':"popup"')

                if popup_location != -1:
                    tabControl.tab(5,text = 'Popup*')
                    sliced_popup = current_attr[popup_location-7:]
                    sliced_popup_end =sliced_popup.find(',{')

                    if sliced_popup_end >=0:
                        sliced_popup = sliced_popup[:sliced_popup_end]

                    popupjson = jsonitize(sliced_popup)

                    popupsrc_entry.delete(0,'end')
                    popupsrc_entry.insert(0,popupjson["src"])

                    popup_xpos_entry.delete(0,'end')
                    popup_xpos_entry.insert(0,popupjson["x"])

                    popup_ypos_entry.delete(0,'end')
                    popup_ypos_entry.insert(0,popupjson["y"])

                    popupwidth_entry.delete(0,'end')
                    popupwidth_entry.insert(0,popupjson["width"])

                    popupheight_entry.delete(0,'end')
                    popupheight_entry.insert(0,popupjson["height"])

class DisplayRotate(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))

            if current_attr != None:
                rotate_location = current_attr.find(':"rotate"')
                if rotate_location != -1:
                    tabControl.tab(6,text = 'Rotate*')
                    sliced_rotate = current_attr[rotate_location-7:]
                    sliced_rotate_end = sliced_rotate.find(',{')
                    if sliced_rotate_end >= 0:
                        sliced_rotate = sliced_rotate[:sliced_rotate_end]

                    rotatejson = jsonitize(sliced_rotate)

                    rotatemax_entry.delete(0,'end')
                    rotatemax_entry.insert(0,rotatejson["max"])

                    rotatemin_entry.delete(0,'end')
                    rotatemin_entry.insert(0,rotatejson["min"])

                    rotatetag_entry.delete(0,'end')
                    rotatetag_entry.insert(0,rotatejson["tag"] )

class DisplayScript(inkex.EffectExtension):
    def effect(self):
        for node in self.svg.selected:
            current_attr = node.get(inkex.addNS('label','inkscape'))
            # inkex.errormsg(current_attr)
            if current_attr != None and current_attr.find('{"attr":"script","list"') != -1:
                tabControl.tab(7,text = 'Script*')
                script_location = current_attr.find('{"attr":"script"')

                sliced_script = current_attr[script_location:]

                find_list = sliced_script.find('"list":')
                find_script_end = sliced_script.find('"}]}')

                get_list = sliced_script[find_list + 7 : find_script_end + 3]
                # inkex.errormsg(("Get LIST:", get_list))
                # inkex.errormsg(("Type Get LIST:", type(get_list)))
                listjson = jsonitize(get_list)

                for i in range(len(listjson)):
                    mouse = 0 #mouse flag
                    for value in listjson[i].values(): #listjson become ditc
                        if "mouseup" in listjson[i].values() and mouse != 1:
                            scripttexts[0].delete('1.0',tk.END)
                            scripttexts[0].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag

                        if "mousedown" in listjson[i].values() and mouse != 1:
                            scripttexts[1].delete('1.0',tk.END)
                            scripttexts[1].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag

                        if "mouseover" in listjson[i].values() and mouse != 1:
                            scripttexts[2].delete('1.0',tk.END)
                            scripttexts[2].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag

                        if "mouseout" in listjson[i].values() and mouse != 1:
                            scripttexts[3].delete('1.0',tk.END)
                            scripttexts[3].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag

                        if "mousemove" in listjson[i].values() and mouse != 1:
                            scripttexts[4].delete('1.0',tk.END)
                            scripttexts[4].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag

                        if "keydown" in listjson[i].values() and mouse != 1:
                            get_keys = listjson[i].get("keys")
                            keyslist = get_keys.split('+')
                            for item in keyslist:
                                if 'ctrl' in item:
                                    keydownCtrl_var.set(1)
                                elif 'alt' in item:
                                    keydownAlt_var.set(1)
                                elif 'shift' in item:
                                    keydownShift_var.set(1)
                                else:
                                    keydownKey_entry.delete(0,tk.END)
                                    keydownKey_entry.insert(0,keyslist[len(keyslist)-1])
                            keydown_text.delete('1.0',tk.END)
                            keydown_text.insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag

                        if "exec_once" in listjson[i].values() and mouse != 1:
                            scripttexts[6].delete('1.0',tk.END)
                            scripttexts[6].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag

                        if "exec_on_update" in listjson[i].values() and mouse != 1:
                            scripttexts[7].delete('1.0',tk.END)
                            scripttexts[7].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag
                        
                        if "vega-lite" in listjson[i].values() and mouse != 1:
                            scripttexts[8].delete('1.0',tk.END)
                            scripttexts[8].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag
                        
                        if "vega" in listjson[i].values() and mouse != 1:
                            scripttexts[9].delete('1.0',tk.END)
                            scripttexts[9].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag
                        
                        if "vega-json" in listjson[i].values() and mouse != 1:
                            scripttexts[10].delete('1.0',tk.END)
                            scripttexts[10].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag
                        
                        if "vega4" in listjson[i].values() and mouse != 1:
                            scripttexts[11].delete('1.0',tk.END)
                            scripttexts[11].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag
                        
                        if "vega4-json" in listjson[i].values() and mouse != 1:
                            scripttexts[12].delete('1.0',tk.END)
                            scripttexts[12].insert('1.0',listjson[i].get("param"))
                            mouse = 1   #mouse flag

            elif current_attr != None and current_attr.find('{"attr":"script","evt"') != -1:
                script_location = current_attr.find('{"attr":"script"')

                find_event = sliced_script.find('"evt":')
                find_script_end = sliced_script.find('"}]}')

                get_list = sliced_script[find_event + 7 : find_script_end + 3]
                # inkex.errormsg(("Get LIST:", get_list))
                # inkex.errormsg(("Type Get LIST:", type(get_list)))

                listjson = jsonitize(sliced_script)
                # inkex.errormsg(("Get JSON:", listjson))
                # inkex.errormsg(("Get JSON len :", range(len(listjson))))

                for i in range(len(script_event)-1):
                    if listjson['evt'] == script_event[i]:
                        scripttexts[i].delete('1.0',tk.END)
                        scripttexts[i].insert('1.0',listjson["param"])

class DisplayGet(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))
            if current_attr != None:
                if (current_attr.find('{"align":"Left","attr":"get"') != -1) or (current_attr.find('{"align":"Right","attr":"get"')!= -1):
                    tabControl.tab(8,text = 'Get*')
                    if current_attr.find('{"align":"Left","attr":"get"') != -1:
                        find_getattr_location = current_attr.find('{"align":"Left","attr":"get"')

                    elif current_attr.find('{"align":"Right","attr":"get"')!= -1:
                        find_getattr_location = current_attr.find('{"align":"Right","attr":"get"')

                    sliced_get = current_attr[find_getattr_location:]
                    sliced_get_end = sliced_get.find(',{')

                    if sliced_get_end >= 0:
                        sliced_get = sliced_get[:sliced_get_end]

                    getjson = jsonitize(sliced_get)

                    if getjson["align"] == "Left":
                        getalign_combo.current(1)
                    elif getjson["align"] == "Right":
                        getalign_combo.current(0)

                    if getjson["type"] == "Live":
                        gettype_combo.current(1)
                    elif getjson["type"] == "Good":
                        gettype_combo.current(0)

                    gettag_entry.delete(0,'end')
                    gettag_entry.insert(0,getjson["tag"])

class DisplaySet(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))
            if current_attr != None:
                if (current_attr.find('{"align":"Left","attr":"set"') != -1) or (current_attr.find('{"align":"Right","attr":"set"')!= -1):
                    tabControl.tab(9,text = 'Set*')
                    if current_attr.find('{"align":"Left","attr":"set"') != -1:
                        find_setattr_location = current_attr.find('{"align":"Left","attr":"set"')

                    elif current_attr.find('{"align":"Right","attr":"set"')!= -1:
                        find_setattr_location = current_attr.find('{"align":"Right","attr":"set"')

                    sliced_set = current_attr[find_setattr_location:]
                    sliced_set_end = sliced_set.find(',{')

                    if sliced_set_end >= 0:
                        sliced_set = sliced_set[:sliced_set_end]

                    setjson = jsonitize(sliced_set)

                    if setjson["align"] == "Left":
                        setalign_combo.current(1)
                    elif setjson["align"] == "Right":
                        setalign_combo.current(0)

                    if setjson["type"] == "Variable":
                        settype_combo.current(1)
                    elif setjson["type"] == "Data":
                        settype_combo.current(0)

                    settag_entry.delete(0,'end')
                    settag_entry.insert(0,setjson["tag"])

                    setprompt_entry.delete(0,'end')
                    setprompt_entry.insert(0,setjson["prompt"])

                    setsource_entry.delete(0,'end')
                    setsource_entry.insert(0,setjson["src"])

class DisplaySlider(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))
            if current_attr != None:
                slider_location = current_attr.find(':"slider"')

                if slider_location != -1:
                    tabControl.tab(10,text = 'Slider*')
                    sliced_slider = current_attr[slider_location-7:]
                    sliced_slider_end = sliced_slider.find(',{')

                    if sliced_slider_end >= 0:
                        sliced_slider = sliced_slider[:sliced_slider_end]

                    sliderjson = jsonitize(sliced_slider)
                    # inkex.errormsg(("JSON",sliderjson))

                    slidermax_entry.delete(0,'end')
                    slidermax_entry.insert(0,sliderjson["max"])

                    slidermin_entry.delete(0,'end')
                    slidermin_entry.insert(0,sliderjson["min"])

                    slidertag_entry.delete(0,'end')
                    slidertag_entry.insert(0,sliderjson["tag"])

                    if sliderjson["readonly"] == 1:
                        sliderread_var.set(1)
                    elif sliderjson["readonly"] == 0:
                        sliderread_var.set(0)

class DisplayTooltips(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))

            if current_attr != None:
                tooltips_location = current_attr.find('{"attr":"tooltips"')
                if tooltips_location != -1:
                    tabControl.tab(11,text = 'Tooltips*')
                    sliced_tooltips = current_attr[tooltips_location:]
                    sliced_tooltips_end = sliced_tooltips.find(',{')

                    if sliced_tooltips_end >= 0:
                        sliced_tooltips = sliced_tooltips[:sliced_tooltips_end]

                    tooltipsjson = jsonitize(sliced_tooltips)

                    paramtooltips = tooltipsjson['param']

                    #inkex.errormsg(type(paramtooltips))

                    for i in range(len(paramtooltips)):
                        tooltipsentries[i].delete(0,'end')
                        tooltipsentries[i].insert(0,paramtooltips[i])

                    tooltipssize_entry.delete(0,'end')
                    tooltipssize_entry.insert(0,tooltipsjson['size'])

                    tooltipsstyle_entry.delete(0,'end')
                    tooltipsstyle_entry.insert(0,tooltipsjson['style'])

class DisplayZoom(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))
            # inkex.errormsg(("Current ",current_attr))
            if current_attr != None and current_attr.find('"attr":"zoom"') != -1:
                tabControl.tab(12,text = 'Zoom*')
                if current_attr.find('{"align":"Center","attr":"zoom"') != -1:
                    find_zoomattr_location = current_attr.find('{"align":"Center","attr":"zoom"')

                elif current_attr.find('{"align":"Left","attr":"zoom"') != -1:
                    find_zoomattr_location = current_attr.find('{"align":"Left","attr":"zoom"')

                elif current_attr.find('{"align":"Right","attr":"zoom"')!= -1:
                    find_zoomattr_location = current_attr.find('{"align":"Right","attr":"zoom"')

                elif current_attr.find('{"align":"Top","attr":"zoom"')!= -1:
                    find_zoomattr_location = current_attr.find('{"align":"Top","attr":"zoom"')

                elif current_attr.find('{"align":"Bottom","attr":"zoom"')!= -1:
                    find_zoomattr_location = current_attr.find('{"align":"Bottom","attr":"zoom"')

                sliced_zoom = current_attr[find_zoomattr_location:]
                sliced_zoom_end = sliced_zoom.find(',{')

                if sliced_zoom_end >= 0:
                    sliced_zoom = sliced_zoom[:sliced_zoom_end]

                zoomjson = jsonitize(sliced_zoom)
                # inkex.errormsg(("JSON",zoomjson))

                if zoomjson["align"] == "Center":
                    zoomalign_combo.current(1)
                elif zoomjson["align"] == "Left":
                    zoomalign_combo.current(2)
                elif zoomjson["align"] == "Right":
                    zoomalign_combo.current(3)
                elif zoomjson["align"] == "Top":
                    zoomalign_combo.current(4)
                elif zoomjson["align"] == "Bottom":
                    zoomalign_combo.current(5)

                zoomtag_entry.delete(0,'end')
                zoomtag_entry.insert(0,zoomjson["tag"])

class DisplayText(inkex.EffectExtension):
    def effect(self):

        for node in self.svg.selected:

            current_attr = node.get(inkex.addNS('label','inkscape'))
            if current_attr != None:
                text_location = current_attr.find('{"attr":"text"')
                if text_location != -1:
                    tabControl.tab(13,text = 'Text*')
                    sliced_text = current_attr[text_location:]
                    sliced_text_end = sliced_text.find(',{')

                    if sliced_text_end >= 0:
                        sliced_text = sliced_text[:sliced_text_end]

                    textjson = jsonitize(sliced_text)
                    # inkex.errormsg(("After",sliced_opac))

                    maptext = textjson['map']

                    removedequal_list=[]
                    removedequal_odd_list = []
                    removedequal_even_list = []

                    for i in maptext:                         #remove the '=' the the list
                        removedequal_list += i.split('=')

                    for s in range(0, len(removedequal_list)):  #split the list into two lists which are odd index and even index
                        if ( s % 2 ):
                            removedequal_even_list.append(removedequal_list[s])
                        else:
                            removedequal_odd_list.append(removedequal_list[s])

                    for tv in range(len(removedequal_odd_list)):# Display the odd index list ar Tag Value column
                        text_TagValues[tv].delete(0,'end')
                        text_TagValues[tv].insert(0,removedequal_odd_list[tv])

                    for tt in range(len(removedequal_even_list)):# Display the odd index list ar Tag Value column
                        text_TagTexts[tt].delete(0,'end')
                        text_TagTexts[tt].insert(0,removedequal_even_list[tt])

                    texttag_entry.delete(0,'end')
                    texttag_entry.insert(0,textjson["tag"])

class Bar(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            if isinstance(node, Rectangle):
                global attr
                
                animation_name = "bar"
                bartag = bartag_entry.get()
                barmin = barmin_entry.get()
                barmax = barmax_entry.get()

                if len(bartag) > 0:    #Foolproof for NO Tag Input

                    if len(bartag) != 0 and barmin.isdigit() == True and barmax.isdigit() == True:
                        attr = '{"attr":"' + animation_name + '","max":' + barmax + ',"min":' + barmin + ',"tag":"' + bartag + '"}'
                    if barmin.isdigit() == False:
                        attr = '{"@ttr":"' + animation_name + '","max":' + barmax + ',"min":0,"tag":"' + bartag + '"}'
                    if barmax.isdigit() == False:
                        attr = '{"@ttr":"' + animation_name + '","max":0,"min":' + barmin + ',"tag":"' + bartag + '"}'
                    if barmin.isdigit() == False and barmax.isdigit() == False :
                        attr = '{"@ttr":"' + animation_name + '","max":0,"min":0,"tag":"' + bartag + '"}'

                    animalist.append(attr)

class Faceplate(inkex.EffectExtension):
    def effect(self):
        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            if isinstance(node, Group):
                try:
                    global attr
                    animation_name = "clone"
                    get_allfaceplatelist = []
                    f = json.loads('{"map":""}')
                    x = faceplate_tree.get_children()
                    for child in faceplate_tree.get_children(): #how many column will get how much child
                        l = ['%'] + faceplate_tree.item(child)["values"]
                        if (len(l[1]) > 0 and len(str(l[3])) > 0): # Used to determine whether the valid of written attribute
                            faceplateATTR = True
                        elif (len(l[1]) == 0 and len(str(l[3])) > 0) or (len(l[1]) > 0 and len(str(l[3])) == 0):
                            faceplateATTR = False
                        l_to_str = ''.join(str(elem) for elem in l)
                        get_allfaceplatelist.append(l_to_str)
                    # inkex.errormsg(("get all ",get_allfaceplatelist))

                    get_allfaceplatelist = [ f for f in get_allfaceplatelist if f != "%="]
                    
                    if faceplateATTR == True:
                        attr =  json.loads('{"attr":"","map":""}')
                        attr["attr"] = animation_name
                        attr["map"] = get_allfaceplatelist
                    elif faceplateATTR == False:
                        attr =  json.loads('{"@ttr":"","map":""}')
                        attr["@ttr"] = animation_name
                        attr["map"] = get_allfaceplatelist

                    attr = str(attr)
                    attr = attr.replace(' ','').replace("'",'"')
                    animalist.append(attr)
                except:
                    pass

class Color(inkex.EffectExtension):
    def effect(self):
        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            try:
                global attr
                animation_name = "color"
                get_allcolorlist = []
                c = json.loads('{"data":"","param":"","tag":""}')
                for child in color_tree.get_children(): #how many column will get how much child
                    l = color_tree.item(child)["values"]
                    c["tag"] = l[0]
                    c["data"] = str(l[1])
                    c["param"] = l[2]
                    #Used to check the data has been enter valid completly
                    if len(c["tag"]) != 0 or len(c["data"]) != 0 or len(c["param"]) != 0:
                        colorATTR = False
                        get_allcolorlist.append(c.copy())
                        if len(c["tag"]) > 0 and len(c["data"]) > 0 and len(c["param"]) > 0:
                            colorATTR = True

                if colorATTR == True: # the data is complete
                    attr =  json.loads('{"attr":"","list":""}')
                    attr["attr"] = animation_name
                    attr["list"] = get_allcolorlist
                elif colorATTR == False: #the data is incomplete
                    attr =  json.loads('{"@ttr":"","list":""}')
                    attr["@ttr"] = animation_name
                    attr["list"] = get_allcolorlist

                attr = str(attr)              
                attr = attr.replace(' ','').replace("'",'"')
                animalist.append(attr)
            except:
                pass

class Opacity(inkex.EffectExtension):

    def effect(self):
        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)

            global attr
            
            animation_name = "opac"
            opactag = opactag_entry.get()
            opacmin = opacmin_entry.get()
            opacmax = opacmax_entry.get()

            if len(opactag) > 0: #Foolproof for NO Tag INPUT

                if len(opactag) != 0 and opacmin.isdigit() == True and opacmax.isdigit() == True:
                    attr = '{"attr":"' + animation_name + '","max":' + opacmax + ',"min":' + opacmin + ',"tag":"' + opactag + '"}'
                if opacmin.isdigit() == False :
                    attr = '{"@ttr":"' + animation_name + '","max":' + opacmax + ',"min":0,"tag":"' + opactag + '"}'
                if opacmax.isdigit() == False :
                    attr = '{"@ttr":"' + animation_name + '","max":0,"min":' + opacmin + ',"tag":"' + opactag + '"}'
                if opacmin.isdigit() == False and opacmax.isdigit() == False :
                    attr = '{"@ttr":"' + animation_name + '","max":0,"min":0,"tag":"' + opactag + '"}'

                animalist.append(attr)

class Open(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            if not isinstance(node, TextElement):

                global attr
                animation_name = "open"
                openheight = openheight_entry.get()
                opensource = opensource_entry.get()

                if opensource_type_combo.get() == 'Tag':
                    opensource_type = '1'
                elif opensource_type_combo.get() == 'URL':
                    opensource_type = '0'

                if opendest_type_combo.get() == 'Current Window':
                    opendest_type = "_self"
                elif opendest_type_combo.get() == 'New exclusive window':
                    opendest_type = "_blank"
                elif opendest_type_combo.get() == 'New shared window':
                    opendest_type = "_shared"

                openwidth = openwidth_entry.get()
                openx = open_xpos_entry.get()
                openy = open_ypos_entry.get()
                old_attr = node.get(inkex.addNS('label','inkscape'))

                if len(opensource) > 0: #Foolproof for NO Source INPUT

                    if len(opensource) != 0 and openheight.isdigit() == True and openwidth.isdigit() == True and openx.isdigit() == True and openy.isdigit() == True:
                        attr = '{"attr":"' + animation_name + '","height":' + openheight + ',"istag":' + opensource_type \
                        + ',"src":"' + opensource + '","type":"' + opendest_type + '","width":' + openwidth + ',"x":' \
                        + openx +',"y":' + openy  + '}'
                    else:
                        if openx.isdigit() == False:
                            openx = '0'
                        if openy.isdigit() == False:
                            openy = '0'
                        if openheight.isdigit() == False:
                            openheight ='0'
                        if openwidth.isdigit() == False:
                            openwidth = '0'

                        attr = '{"@ttr":"' + animation_name + '","height":' + openheight + ',"istag":' + opensource_type \
                        + ',"src":"' + opensource + '","type":"' + opendest_type + '","width":' + openwidth + ',"x":' \
                        + openx +',"y":' + openy  + '}'

                    animalist.append(attr)

class Popup(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)

            global attr
            animation_name = "popup"
            popupsrc = popupsrc_entry.get()
            popup_x = popup_xpos_entry.get()
            popup_y = popup_ypos_entry.get()
            popupwidth = popupwidth_entry.get()
            popupheight = popupheight_entry.get()

            if len(str(popupsrc)) > 0: #Foolproof for NO Source INPUT

                if len(popupsrc) != 0 and popup_x.isdigit() == True and popup_y.isdigit() == True and popupwidth.isdigit() == True and popupheight.isdigit() == True:
                    attr = '{"attr":"' + animation_name + '","height":' + popupheight + ',"src":"' + popupsrc + '","width":' + popupwidth + ',"x":' + popup_x + ',"y":' + popup_y + '}'
                else:
                    if popup_x.isdigit() == False:
                        popup_x = '0'
                    if popup_y.isdigit() == False:
                        popup_y = '0'
                    if popupwidth.isdigit() == False:
                        popupwidth = '0'
                    if popupheight.isdigit() == False:
                        popupheight = '0'
                    attr = '{"@ttr":"' + animation_name + '","height":' + popupheight + ',"src":"' + popupsrc + '","width":' + popupwidth + ',"x":' + popup_x + ',"y":' + popup_y + '}'

                animalist.append(attr)
            
class Rotate(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)

            global attr
            animation_name = "rotate"
            rotatetag = rotatetag_entry.get()
            rotatemin = rotatemin_entry.get()
            rotatemax = rotatemax_entry.get()

            if len(str(rotatetag)) > 0: #Foolproof for NO Tag INPUT

                if len(rotatetag) != 0 and rotatemin.isdigit() == True and rotatemax.isdigit() == True:
                    attr = '{"attr":"' + animation_name + '","max":' + rotatemax + ',"min":' + rotatemin + ',"tag":"' + rotatetag + '"}'
                if rotatemin.isdigit() == False:
                    attr = '{"@ttr":"' + animation_name + '","max":' + rotatemax + ',"min":0,"tag":"' + rotatetag + '"}'
                if rotatemax.isdigit() == False:
                    attr = '{"@ttr":"' + animation_name + '","max":0,"min":' + rotatemin + ',"tag":"' + rotatetag + '"}'
                if rotatemin.isdigit() == False and rotatemax.isdigit() == False:
                    attr = '{"@ttr":"' + animation_name + '","max":0,"min":0,"tag":"' + rotatetag + '"}'

                animalist.append(attr)

class Script(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            try:

                global attr
                animation_name = "script"
                global scriptlist
                scriptlist = ''
                for st in range(len(scripttexts)):

                    if len(scripttexts[st].get("1.0",tk.END)) > 1:
                        scriptevent = ''.join(script_tree.item(st,"value"))
                        scriptparam = scripttexts[st].get("1.0",tk.END)

                        while scriptparam.endswith('\n'):
                            scriptparam = scriptparam[:-1]

                        if scriptlist == '':
                            scriptlist = '{"evt":"' + scriptevent + '","param":"' + scriptparam +'"}'
                        else:
                            scriptlist = scriptlist + "," + '{"evt":"' + scriptevent + '","param":"' + scriptparam +'"}'

                if len(keydown_text.get("1.0",tk.END)) > 1:

                    keydownevent = ''.join(script_tree.item(5,"value"))
                    keydownparam = keydown_text.get("1.0",tk.END)

                    while keydownparam.endswith('\n'):
                        keydownparam = keydownparam[:-1]

                    global keydownkeyslist
                    temp_keydownkeyslist = []
                    keydownkeyslist = []

                    if keydownCtrl_var.get() == 1:
                        temp_keydownkeyslist.append('ctrl')
                    if keydownAlt_var.get() == 1:
                        temp_keydownkeyslist.append('alt')
                    if keydownShift_var.get() == 1:
                        temp_keydownkeyslist.append('shift')
                    if len(keydownKey_var.get()) >= 1:
                        temp_keydownkeyslist.append(keydownKey_var.get())

                    for k in range(len(temp_keydownkeyslist)):
                        keydownkeyslist.append(temp_keydownkeyslist[k])
                        keydownkeyslist.append('+')

                    if keydownkeyslist[len(keydownkeyslist)-1] == '+':
                        keydownkeyslist.pop(-1)

                    keydownkeys = ''.join([str(elem) for elem in keydownkeyslist])

                    if scriptlist == '':
                        scriptlist = '{"evt":"' + keydownevent + '","keys":"' + keydownkeys + '","param":"' + keydownparam +'"}'
                    else:
                        scriptlist = scriptlist + "," + '{"evt":"' + keydownevent + '","keys":"' + keydownkeys + '","param":"' + keydownparam +'"}'

                if len(scriptlist) != 0:
                    attr = '{"attr":"' + animation_name + '","list":[' + scriptlist + ']}'

                    animalist.append(attr)
            except:
                pass

class Get(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            if isinstance(node, TextElement):
                global attr
                animation_name = "get"
                gettag = gettag_entry.get()
                getalign = getalign_combo.get()
                gettype = gettype_combo.get()

                if len(gettag) != 0:
                    attr = '{"align":"' + getalign + '","attr":"' + animation_name + '","tag":"' + gettag + '","type":"' + gettype + '"}'

                    animalist.append(attr)

class Set(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            global attr
            animation_name = "set"
            settag = settag_entry.get()
            setalign = setalign_combo.get()
            setprompt = setprompt_entry.get()
            setsource = setsource_entry.get()
            settype = settype_combo.get()

            if len(settag) != 0:
                attr = '{"align":"' + setalign + '","attr":"' + animation_name + '","prompt":"' + setprompt + '","src":"' + setsource + '","tag":"' + settag + '","type":"' + settype + '"}'

                animalist.append(attr)

class Slider(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)

            global attr
            animation_name = "slider"
            slidertag = slidertag_entry.get()
            slidermin = slidermin_entry.get()
            slidermax = slidermax_entry.get()
            sliderread = sliderread_var.get()

            if len(slidertag) > 0: #Foolproof for NO Tag INPUT

                if len(slidertag) !=0 and slidermin.isdigit() == True and slidermax.isdigit() == True:
                    attr = '{"attr":"' + animation_name + '","max":' + str(slidermax) + ',"min":' + str(slidermin) + ',"readonly":' + str(sliderread) +',"tag":"' + slidertag + '"}'
                if slidermin.isdigit() == False:
                    attr = '{"@ttr":"' + animation_name + '","max":' + str(slidermax) + ',"min":0,"readonly":' + str(sliderread) +',"tag":"' + slidertag + '"}'
                if slidermax.isdigit() == False:
                    attr = '{"@ttr":"' + animation_name + '","max":0,"min":' + str(slidermin) + ',"readonly":' + str(sliderread) +',"tag":"' + slidertag + '"}'
                if slidermax.isdigit() == False and slidermin.isdigit() == False:
                    attr = '{"attr":"' + animation_name + '","max":0,"min":0,"readonly":' + str(sliderread) +',"tag":"' + slidertag + '"}'

                animalist.append(attr)

class Tooltips(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            try:
                global attr

                entry_list = ''
                animation_name = "tooltips"
                for entries in tooltipsentries:
                    if entries.get() != '':
                        entry_list = entry_list + '"' + str(entries.get()) + '",'
                entry_list = entry_list[:-1]

                tooltipssize = tooltipssize_entry.get()
                if not tooltipssize:    #foolproof size of 12 when empty SIZE var input
                    tooltipssize = "12"
                tooltipsstyle = tooltipsstyle_entry.get()

                if len(entry_list) != 0:
                    attr = '{"attr":"' + animation_name + '","param":[' + entry_list + '],"size":' + tooltipssize + ',"style":"' + tooltipsstyle + '"}'

                    animalist.append(attr)

            except ValueError:
                pass

class Zoom(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            global attr
            animation_name = "zoom"
            zoomtag = zoomtag_entry.get()
            zoomalign = zoomalign_combo.get()

            if len(zoomtag) > 0: #Foolproof for NO Tag INPUT

                if zoomalign != 'None':

                    attr = '{"align":"' + zoomalign + '","attr":"' + animation_name + '","tag":"' + zoomtag  + '"}'
                    animalist.append(attr)

class Text(inkex.EffectExtension):

    def effect(self):

        for node in self.svg.selected:
            new_id = object_id_entry.get()
            node.set("id",new_id)
            global attr
            animation_name = "text"

            entry_list = ''
            texttag = texttag_entry.get()

            count = 0
            for tventries in text_TagValues:
                if tventries.get() != '' and count < len(text_TagValues):
                    for ttentries in text_TagTexts:
                        # inkex.errormsg(("len:" , len(text_TagTexts)))
                        if (ttentries.get() == text_TagTexts[count].get()) and ttentries.get() != '' and count < len(text_TagTexts):
                            entry_list = entry_list + '"' + str(text_TagValues[count].get()) + '=' + str(text_TagTexts[count].get()) + '",'
                            count += 1
            entry_list = entry_list[:-1]

            if len(texttag) > 0:
                attr = '{"attr":"' + animation_name + '","map":[' + entry_list + '],"tag":"' + texttag + '"}'
                animalist.append(attr)

class WriteSAGEX(inkex.EffectExtension):
    
    def effect(self):
        # inkex.errormsg("WRITTEN")
        for node in self.svg.selected:
            if len(animalist) > 0:
                animalist_to_str = ','.join([str(elem) for elem in animalist])
                node.set(inkex.addNS('label','inkscape'),animalist_to_str)   

if __name__ == '__main__':

    root.columnconfigure(1, weight =1)
    root.rowconfigure(1, weight =1)

    object_id_label.grid(row = 0, column = 0)
    object_id_entry.grid(row = 0, column = 1,sticky ='WE' ,padx = 12)

    #region BAR tab setting
    bartab.columnconfigure(1, weight =1)

    bartag_label.grid(row = 1, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    bartag_entry.grid(row = 1, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    barmin_label.grid(row = 2, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    barmin_entry.grid(row = 2, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    barmax_label.grid(row = 3, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    barmax_entry.grid(row = 3, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    #endregion

    #region FACEPLATE tab setting
    faceplatetab.columnconfigure(1, weight =1)
    faceplatetab.rowconfigure(0, weight = 1)

    faceplate_variable_label.grid(row = 0, column=0, pady= 5)
    faceplate_value_label.grid(row = 0,column=1, pady= 5)

    faceplate_variable_entry.grid(row = 1, column=0,  sticky ='WE', padx =4, pady = 5)
    faceplate_value_entry.grid(row = 1,column=1,  sticky ='WE', padx =4, pady = 5)

    faceplate_frame.columnconfigure(0,weight = 1)
    faceplate_frame.columnconfigure(1,weight = 1)

    faceplate_frame.rowconfigure(2,weight = 1)

    #ADD faceplate
    def add_faceplate():
        global faceplate_count
        faceplate_count = len(faceplate_tree.get_children())
        faceplate_tree.insert(parent='', index='end', iid=faceplate_count,text="",\
                         values = (faceplate_variable_entry.get(),'=',faceplate_value_entry.get()))
        faceplate_count += 1

        # Clear the entry boxes
        faceplate_variable_entry.delete(0,tk.END)
        faceplate_value_entry.delete(0,tk.END)

    #DELETE faceplate
    def delete_faceplate():
        delete = faceplate_tree.selection()
        for record in delete:
            faceplate_tree.delete(record)

    #UPDATE color
    def update_faceplate():
        selected = faceplate_tree.focus()
        faceplate_tree.item(selected, text='', values=(faceplate_variable_entry.get(),'=',faceplate_value_entry.get()))

        # Clear the entry boxes
        faceplate_variable_entry.delete(0,tk.END)
        faceplate_value_entry.delete(0,tk.END)

    def up_faceplate():
        faceplate_rows = faceplate_tree.selection()
        for faceplate_row in faceplate_rows:
            faceplate_tree.move(faceplate_row, faceplate_tree.parent(faceplate_row), faceplate_tree.index(faceplate_row)-1)

    def down_faceplate():
        faceplate_rows = faceplate_tree.selection()
        for faceplate_row in reversed(faceplate_rows):
            faceplate_tree.move(faceplate_row, faceplate_tree.parent(faceplate_row), faceplate_tree.index(faceplate_row)+1)

    def select_face():
        try:
            faceplate_variable_entry.delete(0,tk.END)
            faceplate_value_entry.delete(0,tk.END)

            # Grab record number
            selected_face = faceplate_tree.focus()
            # Grab record values
            face_values = faceplate_tree.item(selected_face,'values')

            #output to entry boxes
            faceplate_variable_entry.insert(0, face_values[0])
            faceplate_value_entry.insert(0,face_values[2])
        except IndexError:
            pass

    def face_clicked(event):
        select_face()

    faceplate_button_frame = tk.Frame(faceplate_frame)
    faceplate_button_frame.grid(row=1, rowspan=3, column=3, sticky="N", pady = 3)

    #BUTTON
    add_button = tk.Button(faceplate_button_frame, text = 'Add', command = add_faceplate,  width=9)
    update_button = tk.Button(faceplate_button_frame, text = 'Update', command = update_faceplate, width=9)
    up_button = tk.Button(faceplate_button_frame, text = 'Move Up', command = up_faceplate, width=9)
    down_button = tk.Button(faceplate_button_frame, text = 'Move Down', command = down_faceplate, width=9)
    delete_button = tk.Button(faceplate_button_frame, text = 'Delete', command = delete_faceplate, width=9)

    add_button.grid(row=0,column = 3, sticky='WE', pady= 10, padx =4)
    update_button.grid(row=1, column = 3, sticky='WE', pady = 10, padx =4)
    up_button.grid(row=2, column = 3, sticky='WE', pady= 10, padx =4)
    down_button.grid(row=3, column = 3, sticky='WE', pady= 10, padx =4)
    delete_button.grid(row=4, column = 3, sticky='WE', pady= 10, padx =4)

    faceplate_tree.bind("<ButtonRelease-1>",face_clicked)
    #endregion

    #region COLOR tab setting
    colortab.columnconfigure(1, weight =1)
    colortab.rowconfigure(0, weight = 1)

    color_variable_label.grid(row = 0, column=0, pady= 5)
    color_limit_label.grid(row = 0,column=1, pady= 5)
    color_name_code_label.grid(row = 0, column = 2, pady= 5)

    color_variable_entry.grid(row = 1, column=0, padx= 3, sticky ='WE')
    color_limit_entry.grid(row = 1,column=1, padx= 3, sticky ='WE')
    color_name_code_entry.grid(row = 1,column=2, padx= 3, sticky ='WE')

    color_frame.columnconfigure(0,weight = 1)
    color_frame.columnconfigure(1,weight = 1)
    color_frame.columnconfigure(2,weight = 1)

    color_frame.rowconfigure(2,weight = 1)
    color_frame.rowconfigure(3,weight = 1)
    color_frame.rowconfigure(4,weight = 1)
    color_frame.rowconfigure(5,weight = 1)

    #ADD color
    def add_color():

        global color_count
        color_count = len(color_tree.get_children())
        color_tree.insert(parent='', index='end', iid=color_count,text="",\
                         values = (color_variable_entry.get(),color_limit_entry.get(),color_name_code_entry.get()))
        color_count += 1

        # Clear the entry boxes
        color_variable_entry.delete(0,tk.END)
        color_limit_entry.delete(0,tk.END)
        color_name_code_entry.delete(0,tk.END)

    #DELETE color
    def delete_color():
        delete = color_tree.selection()
        for record in delete:
            color_tree.delete(record)

         # Clear the entry boxes
        color_variable_entry.delete(0,tk.END)
        color_limit_entry.delete(0,tk.END)
        color_name_code_entry.delete(0,tk.END)

    #UPDATE color
    def update_color():
        selected = color_tree.focus()
        color_tree.item(selected, text='', values=(color_variable_entry.get(),color_limit_entry.get(),color_name_code_entry.get()))

        # Clear the entry boxes
        color_variable_entry.delete(0,tk.END)
        color_limit_entry.delete(0,tk.END)
        color_name_code_entry.delete(0,tk.END)

    def up_color():
        color_rows = color_tree.selection()
        for color_row in color_rows:
            color_tree.move(color_row, color_tree.parent(color_row), color_tree.index(color_row)-1)

    def down_color():
        color_rows = color_tree.selection()
        for color_row in reversed(color_rows):
            color_tree.move(color_row, color_tree.parent(color_row), color_tree.index(color_row)+1)

    def color_picker():
        global colorpicker_flag
        colorpicker_flag = 1
        colorcode = colorchooser.askcolor()[1]
        try:
            if len(color_name_code_entry.get()) != 0:
                color_name_code_entry.insert(len(color_name_code_entry.get()),("/" + colorcode.upper()))
            else:
                color_name_code_entry.insert(0,colorcode.upper())
            colorpicker_flag = 0
        except:
            pass
    latest_color = -1
    s_flag = 0
    def select_color():
        try:
            color_variable_entry.delete(0,tk.END)
            color_limit_entry.delete(0,tk.END)
            color_name_code_entry.delete(0,tk.END)

            # Grab record number
            global latest_color,s_flag
            selected_color = color_tree.focus()
            # Grab record values
            color_values = color_tree.item(selected_color,'values')
            # inkex.errormsg(len(color_values))
            #Output to entry boxes
            color_variable_entry.insert(0,color_values[0])
            color_limit_entry.insert(0,color_values[1])
            color_name_code_entry.insert(0,color_values[2])

            if selected_color == latest_color:
                if s_flag == 0:
                    color_tree.selection_remove(selected_color)
                    s_flag = 1
                else:
                    s_flag = 0
            else:
                s_flag = 0
            latest_color = selected_color
        except IndexError:
            pass

    def color_clicked(event):
        select_color()

    #ADD Frame to put the up, down, update, delete buttons
    color_button_frame = tk.Frame(color_frame)
    color_button_frame.grid(row=0,rowspan=3, column = 3, sticky = "N", pady=20)

    # Button
    colorpicker = tk.Button(color_button_frame, text = 'Color Picker', command = color_picker, width=9)
    add_button = tk.Button(color_button_frame, text = 'Add', command = add_color,  width=9)
    update_button = tk.Button(color_button_frame, text = 'Update', command = update_color, width=9)
    up_button = tk.Button(color_button_frame, text = 'Move Up', command = up_color, width=9)
    down_button = tk.Button(color_button_frame, text = 'Move Down', command = down_color, width=9)
    delete_button = tk.Button(color_button_frame, text = 'Delete', command = delete_color, width=9)

    colorpicker.grid(row=0,column = 0, sticky='WE', pady= 10, padx =4)
    add_button.grid(row = 1,column = 0, sticky='WE', pady= 10, padx =4 )
    update_button.grid(row = 2, column = 0, pady= 10, padx =4)
    up_button.grid(row = 3, column = 0, pady= 10, padx =4)
    down_button.grid(row = 4, column = 0, pady= 10, padx =4)
    delete_button.grid(row = 5, column = 0, pady= 10, padx =4)

    color_tree.bind("<ButtonRelease-1>",color_clicked)
    #endregion

    #region OPACITY tab setting
    opactab.columnconfigure(1, weight =1)

    opactag_label.grid(row = 1, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    opactag_entry.grid(row = 1, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    opacmin_label.grid(row = 2, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    opacmin_entry.grid(row = 2, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    opacmax_label.grid(row = 3, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    opacmax_entry.grid(row = 3, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    #endregion

    #region OPEN tab setting
    opentab.columnconfigure(1, weight =1)

    def disabledopen_xywh(event):
        if opendest_type_var.get() == 'Current Window':
            open_xpos_entry.configure(state="disable")
            open_ypos_entry.configure(state="disable")
            openwidth_entry.configure(state="disable")
            openheight_entry.configure(state="disable")
        else:
            open_xpos_entry.configure(state="normal")
            open_ypos_entry.configure(state="normal")
            openwidth_entry.configure(state="normal")
            openheight_entry.configure(state="normal")

    opendest_type_combo.bind('<<ComboboxSelected>>',disabledopen_xywh)

    opensource_label.grid(row = 0, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    opensource_entry.grid(row = 0, column = 1 ,sticky="EW", padx = 4 , pady = 4)

    opensource_type_label.grid(row = 1, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    opensource_type_combo .grid(row = 1, column = 1 ,sticky="EW", padx = 4 , pady = 4)

    opendest_type_label.grid(row = 2, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    opendest_type_combo.grid(row = 2, column = 1 ,sticky="EW", padx = 4 , pady = 4)

    open_xpos_label.grid(row = 3, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    open_xpos_entry.grid(row = 3, column = 1 ,sticky="EW", padx = 4 , pady = 4)

    open_ypos_label.grid(row = 4, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    open_ypos_entry.grid(row = 4, column = 1 ,sticky="EW", padx = 4 , pady = 4)

    openwidth_label.grid(row = 5, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    openwidth_entry.grid(row = 5, column = 1 ,sticky="EW", padx = 4 , pady = 4)

    openheight_label.grid(row = 6, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    openheight_entry.grid(row = 6, column = 1 ,sticky="EW", padx = 4 , pady = 4)
    #endregion

    #region POPUP tab setting
    popuptab.columnconfigure(1, weight =1)

    popupsrc_label.grid(row = 1, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    popupsrc_entry.grid(row = 1, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    popup_xpos_label.grid(row = 2, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    popup_xpos_entry.grid(row = 2, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    popup_ypos_label.grid(row = 3, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    popup_ypos_entry.grid(row = 3, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    popupwidth_label.grid(row = 4, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    popupwidth_entry.grid(row = 4, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    popupheight_label.grid(row = 5, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    popupheight_entry.grid(row = 5, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    #endregion

    #region ROTATE tab setting
    rotatetab.columnconfigure(1, weight =1)

    rotatetag_label.grid(row = 1, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    rotatetag_entry.grid(row = 1, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    rotatemin_label.grid(row = 2, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    rotatemin_entry.grid(row = 2, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    rotatemax_label.grid(row = 3, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    rotatemax_entry.grid(row = 3, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    #endregion

    #region SCRIPT tab setting
    scripttab.columnconfigure(1, weight =1)
    scripttab.rowconfigure(0, weight = 1)

    scriptframe.columnconfigure(0, weight =1)
    scriptframe.rowconfigure(0, weight =1)

    keydownframe.rowconfigure(1,weight=1)
    keydownframe.columnconfigure(5,weight=1)

    scripttexts[0].grid(row=0, columnspan=3, padx = 4, pady = 4 ,sticky = 'NSWE')

    def mouseevent(event):
        try:
            script_tree_item = script_tree.identify('item',event.x,event.y)

            keydownframe.grid_forget()

            if int(script_tree_item) == 0:
                scripttexts[0].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()

            elif int(script_tree_item) == 1:
                scripttexts[1].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()

            elif int(script_tree_item) == 2:
                scripttexts[2].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()

            elif int(script_tree_item) == 3:
                scripttexts[3].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()

            elif int(script_tree_item) == 4:
                scripttexts[4].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()

            elif int(script_tree_item) == 5:
                keydownframe.grid(row =0, column = 1,  sticky ='NSWE')

                keydownCtrl_checkbox.grid(row=0,column=0, pady = 4)
                keydownAlt_checkbox.grid(row=0,column=1, pady = 4)
                keydownShift_checkbox.grid(row=0,column=2, pady = 4)
                keydownKey_label.grid(row=0,column=3, pady = 4)
                keydownKey_entry.grid(row=0,column=4, pady = 4)
                keydown_text.grid(row=1,columnspan=6,pady = 4, sticky = 'NESW')

            elif int(script_tree_item) == 6:
                scripttexts[6].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()

            elif int(script_tree_item) == 7:
                scripttexts[7].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()

            elif int(script_tree_item) == 8:
                scripttexts[8].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()

            elif int(script_tree_item) == 9:
                scripttexts[9].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()   
                
            elif int(script_tree_item) == 10:
                scripttexts[10].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[11].grid_forget()
                scripttexts[12].grid_forget()                

            elif int(script_tree_item) == 11:
                scripttexts[11].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[12].grid_forget()                

            elif int(script_tree_item) == 12:
                scripttexts[12].grid(row=0, columnspan=3, padx = 4, pady = 4, sticky = 'NSWE')
                scripttexts[0].grid_forget()
                scripttexts[1].grid_forget()
                scripttexts[2].grid_forget()
                scripttexts[3].grid_forget()
                scripttexts[4].grid_forget()
                scripttexts[5].grid_forget()
                scripttexts[6].grid_forget()
                scripttexts[7].grid_forget()
                scripttexts[8].grid_forget()
                scripttexts[9].grid_forget()
                scripttexts[10].grid_forget()
                scripttexts[11].grid_forget()                

        except ValueError:
            return

    script_tree.bind("<ButtonRelease-1>",mouseevent)

    #endregion

    #region GET tab setting
    gettab.columnconfigure(1, weight =1)

    gettag_label.grid(row = 0, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    gettag_entry.grid(row = 0, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    getalign_label.grid(row = 1, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    getalign_combo .grid(row = 1, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    #endregion

    #region SET tab setting
    settab.columnconfigure(1, weight =1)

    settag_label.grid(row = 0, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    settag_entry.grid(row = 0, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    setalign_label.grid(row = 1, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    setalign_combo .grid(row = 1, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    setprompt_label.grid(row = 2, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    setprompt_entry.grid(row = 2, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    setsource_label.grid(row = 3, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    setsource_entry.grid(row = 3, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    settype_label.grid(row = 4, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    settype_combo.grid(row = 4, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    #endregion

    #region SLIDER tab setting
    slidertab.columnconfigure(1, weight =1)

    slidertag_label.grid(row = 1, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    slidertag_entry.grid(row = 1, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    slidermin_label.grid(row = 2, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    slidermin_entry.grid(row = 2, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    slidermax_label.grid(row = 3, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    slidermax_entry.grid(row = 3, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    sliderread_checkbox.grid(row =4, column = 1 ,sticky = "W" , padx = 4 , pady = 4)
    #endregion

    #region TOOLTIPS tab setting
    tooltipstab.columnconfigure(1, weight =1)

    tooltipssize_label.grid(row = 5, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    tooltipssize_entry.grid(row = 5, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    tooltipsstyle_label.grid(row = 6, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    tooltipsstyle_entry.grid(row = 6, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)
    #endregion

    #region ZOOM tab setting
    zoomtab.columnconfigure(1, weight =1)

    zoomtag_label.grid(row = 1, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    zoomtag_entry.grid(row = 1, column = 1 ,sticky="EW", padx = 4 , pady = 4)
    zoomalign_label.grid(row = 0, column = 0 ,sticky="EW", padx = 4 , pady = 4)
    zoomalign_combo .grid(row = 0, column = 1 ,sticky="EW", padx = 4 , pady = 4)

    #endregion

    #region TEXT tab setting
    texttab.columnconfigure(1, weight =1)

    texttag_label.grid(row = 0, column = 0 ,sticky = 'WE' , padx = 4 , pady = 4)
    texttag_entry.grid(row = 0, column = 1 ,sticky = 'WE' , padx = 4 , pady = 4)

    texttag_value_label.grid(row = 0, column = 0, padx = 4, pady = 4, sticky = 'W')
    texttag_text_label.grid(row = 0, column = 2, padx = 4, pady = 4, sticky = 'W')
    #endregion

    #region ABOUT tab setting
    abouttab.columnconfigure(1, weight =1)
    abouttab.rowconfigure(0, weight = 1)

    aboutframe.columnconfigure(0, weight =1)
    aboutframe.rowconfigure(0, weight = 1)

    abouttext.configure(state=tk.DISABLED)
    abouttext.grid(row = 0, column = 0, padx = 4, pady = 4, sticky = 'NSWE')
    #endregion

    DisplayBar().run()
    DisplayFaceplate().run()
    DisplayColor().run()
    DisplayOpacity().run()
    DisplayOpen().run()
    DisplayPopup().run()
    DisplayRotate().run()
    DisplayScript().run()
    DisplayGet().run()
    DisplaySet().run()
    DisplaySlider().run()
    DisplayTooltips().run()
    DisplayZoom().run()
    DisplayText().run()
    SAGEX().run()

# region LOSS FOUCUSING setting

    def lossfocus(event):
        global colorpicker_flag # for color pricker

        try:
            if event.widget is root:
                # check which widget getting the focus
                w = root.tk.call('focus')
                if not w and colorpicker_flag == 0:
                    # not widget in this window
                    Bar().run()
                    Faceplate().run()
                    Color().run()
                    Opacity().run()
                    Open().run()    
                    Popup().run()    
                    Rotate().run()
                    Script().run()     
                    Set().run()   
                    Get().run()   
                    Slider().run()    
                    Tooltips().run()    
                    Zoom().run()  
                    Text().run() 
                    WriteSAGEX().run()
                    # root.destroy() # Don't destroy the window on focus loss
                    pass # Keep the window open
        except NameError:
            root.quit()

    root.bind('<FocusOut>', lossfocus)

    def on_closing(): # Used to flag which tab is using
        Bar().run()
        Faceplate().run()
        Color().run()
        Opacity().run()
        Open().run()    
        Popup().run()    
        Rotate().run()
        Script().run()     
        Set().run()   
        Get().run()   
        Slider().run()    
        Tooltips().run()    
        Zoom().run()  
        Text().run() 
        WriteSAGEX().run()
        root.quit()

    def handle_escape(event):
        """Handles the Escape key press event."""
        on_closing()
#endregion 

    root.protocol("WM_DELETE_WINDOW", on_closing) # This is used to detect the closing button of window
    root.bind('<Escape>', handle_escape) # Bind Escape key to close the window
    root.geometry("540x600")
    root.mainloop()