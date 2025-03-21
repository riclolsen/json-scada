# SVG Synoptic Display Editor

This is a separate project. This editor is based on the popular open-source Inkscape editor, it was modified to allow markup SCADA-like animations in the SVG file with links to data that can be later provided at runtime.
Any graphics properties can be animated with live data, such as fill/stroke colors, size, position, rotation, etc.

## Source Code

- [Display Editor Source Code](https://gitlab.com/ricardolo/inkscape-rebased)

## Installers

Windows binaries are include in the {json:scada} installer for Windows 64 bits.

It can also be acquired here the standalone Microsoft Store version for Windows 10 (Certified Binary).

- [Microsoft Store Binary](https://www.microsoft.com/store/apps/9P9905HMKZ7X?ocid=periscope)

## Creating Displays

To create a new display

1. Create the SVG graphics using the Synoptic Editor. It is possible to use other editors (Illustrator, Corel, etc.) to create graphics that can be imported in the Synoptic Editor to later markup. The editor can import graphics from a great number of formats. There are on the web many sources of vector graphics clipart (free and paid) that can be useful. It is recommended to configure each new display file with a size of 2400 x 1500 pixels (File \| Document Properties \| Page \| Page Size \| Custom Size). This is a reference size, the actual drawing can be larger.

2. Markup the animations you want in the graphics using the Synoptic Editor. For this, select the object you want to animate and click the mouse right button and select “Object Properties”. Then choose from the menu the properties you want to animate. Follow the documentation below to understand the parameters of animations. Always use JSON-SCADA TAGs as identifiers of values to animate the graphics at runtime in the web browser. Finally, save the file always using the default native Inkscape SVG format.

3. If you are editing on the JSON-SCADA server, save the file to "C:\json-scada\src\htdocs\svg" or equivalent folder. Add the file to display lists in the "C:\json-scada\src\htdocs\svg\screen_list.js" file. Open the Display Viewer web browser. The viewer can be also directly opened with a URL like this "http://127.0.0.1:8080/display?SVGFILE=filename.svg".

## Tag naming

Tag names are strings that must always begin with a letter (a-z or A-Z). Tags beginning with a number like “21xyz” will be converted to numbers and will cause problems. Do not begin a tag with any symbol, symbols are reserved for special purposes. If necessary, prefix your tag names with letters to avoid problems.
Point numbers (_id field of \_realtimeData_ collection) also can be used to identify points in the SVG Editor. There is no limit on the size of the tag string. Tag names should be unique across the whole point database.

## Standard Inkscape Usage

There is plenty of material available throughout the web to help learn how to use the Inkscape SVG graphics editor. We just point here some useful resources.

- Inkscape official manual
  [http://tavmjong.free.fr/INKSCAPE/MANUAL/html/](http://tavmjong.free.fr/INKSCAPE/MANUAL/html/)

- Inkscape tutorials, books, videos, etc.
  [https://inkscape.org/en/learn/](https://inkscape.org/en/learn/)

An interactive tutorial is available inside the editor in many languages (menu Help \| Tutorials).

Other useful related resources.

- [https://www.opto22.com/support/resources-tools/demos/svg-image-library](https://www.opto22.com/support/resources-tools/demos/svg-image-library)
- [https://github.com/willianjusten/awesome-svg](https://github.com/willianjusten/awesome-svg)
- [https://github.com/PanderMusubi/inkscape-open-symbols](https://github.com/PanderMusubi/inkscape-open-symbols)
- [https://sourceforge.net/projects/oshmiopensubstationhmi/files/svg-clipart.zip/download](https://sourceforge.net/projects/oshmiopensubstationhmi/files/svg-clipart.zip/download)
- [https://www.svgrepo.com](https://www.svgrepo.com)
- [https://www.vecteezy.com](https://www.vecteezy.com)

## SCADA Animations

To edit SCADA animation properties of an SVG object, right-click the mouse and choose Object Properties (please notice that albeit the Inkscape software interface is localized in many languages, the Object Properties menu is only available in English).

![SVG Editor](https://scadavis.io/images/image2.png 'SVG Editor')
Editor interface showing the “Object Properties” dialog.

Follow below a list of attributes (“Object Properties\|Tab”) that can be utilized to animate graphics.

| Desired Action                           | Tab                                                                                          |
| ---------------------------------------- | -------------------------------------------------------------------------------------------- |
| Show formatted values as text            | [Get](#get-tab)                                                                              |
| Define texts for ranges of values        | [Text](#text-tab)                                                                            |
| Change color of drawing objects          | [Color](#color-tab)                                                                          |
| Change SVG attributes of drawing objects | [Color](#color-tab)                                                                          |
| Load images                              | [Color](#color-tab) \| [Script](#script-tab)                                                 |
| Run animations                           | [Color](#color-tab) \| [Script](#script-tab)                                                 |
| Execute scripts                          | [Color](#color-tab) \| [Script](#script-tab) \| [Set (#exec_once #exec_on_update)](#set-tab) |
| Associate mouse/keyboard events          | [Script](#script-tab)                                                                        |
| Bar graph                                | [Bar](#bar-tab)                                                                              |
| Arc, Donut                               | [Set (#arc)](#set-tab)                                                                       |
| Radar chart                              | [Set (#radar)](#set-tab)                                                                     |
| Generic charts                           | [Script](#script-tab) \| [Set (#vega4 #vega4-json #vega-lite)](#set-tab)                     |
| Control transparency                     | [Opacity](#opacity-tab), [Color](#color-tab)                                                 |
| Rotate objects                           | [Rotate](#rotate-tab)                                                                        |
| Create tooltips (on mouse over text)     | [Tooltips](#tooltips-tab)                                                                    |
| Move objects linearly                    | [Slider](#slider-tab)                                                                        |
| Zoom to area when object clicked         | [Zoom](#zoom-tab)                                                                            |
| Control action when object is clicked    | [Popup](#popup-tab) \| [Open](#open-tab)                                                     |
| Create Trend Plot                        | [Open](#open-tab)                                                                            |
| Open New Display                         | [Open](#open-tab)                                                                            |
| Open URL on new Window                   | [Open](#open-tab) \| [Popup](#popup-tab)                                                     |
| Preview Displays (on mouse over)         | [Open](#open-tab) \| [Popup](#popup-tab)                                                     |
| Create indirect variables                | [Faceplate (group)](#faceplate-tab)                                                          |
| Clone object properties/behavior         | [Set (#copy_xsac_from)](#set-tab)                                                            |
| Object models                            | [Faceplate (group)](#faceplate-tab) + [Set (#copy_xsac_from)](#set-tab)                      |
| Define filter for Alarm Box              | [Set (#set_filter)](#set-tab)                                                                |

### Get Tab

**Purpose**: retrieve and show formatted values for tags.

**Available for**: SVG text objects only.

In the Tag field, put the tag to be retrieved its value. The fields _Alignment_ and _Type_ are ignored. To align text use the Inkscape “Text and Font” menu.

There are 3 ways to format values obtained by the “Get” directive.

When the text of the object contains the “\|” (pipe) character, it is used the **Boolean** convention. When the text contains the “%” character, it is be used the **Printf** convention. In all other cases it is used the **d3** convention.

- **Printf** convention.

  To format values it is possible to use the standard C language _printf_ convention in the text of the object (e.g: “%5.2f”). For analog values, use “%f”. For string values, use “%s”.

  For a complete _printf_ convention reference, see

  - [http://www.cplusplus.com/reference/cstdio/printf/](http://www.cplusplus.com/reference/cstdio/printf/)

  - [https://alvinalexander.com/programming/printf-format-cheat-sheet](https://alvinalexander.com/programming/printf-format-cheat-sheet)

  This convention can be used to format number and string values.

- **d3** convention.

  The **d3** format convention can also be used.
  The character “~” should be used in place of “%” when d3.format percent convention is necessary.
  The default locale is US English to change it, call d3.formatLocale from a script.
  This convention can be used to format only numeric values.

  For a reference see

  - [https://github.com/d3/d3-format](https://github.com/d3/d3-format)

  - [http://bl.ocks.org/zanarmstrong/05c1e95bf7aa16c4768e](http://bl.ocks.org/zanarmstrong/05c1e95bf7aa16c4768e)

- **Boolean** convention.

  For boolean values, use “off_text\|on_text\|failed_text”, to show custom texts based on the tag value and quality.

It's possible to represent flow direction for analog values with an arrow in place of the value signal using the following codes (positioned where you want the arrow to be shown):

- 'u^': up pointing arrow for positive values (down for negative values).
- 'd^': down pointing arrow for positive values (up for negative values).
- 'r^': right pointing arrow for positive values (left for negative values).
- 'l^': left pointing arrow for positive values (right for negative values).
- 'a^': shows only the absolute value.

Examples of formatting using the _printf_ convention. Considering the value = -23.456:

| format  | presented value |
| ------- | --------------- |
| %6.2f   | -23.47          |
| %08.3f  | -023.456        |
| %1.0f   | -23             |
| %5.2fu^ | 23.47↓          |
| l^%.1f  | →23.1           |

Examples of formatting using the _d3_ convention. Considering the value = 123456789.123:

| format | presented value |
| ------ | --------------- |
| s      | 123.456789123M  |
| .3s    | 123M            |

Examples of formatting using the _boolean_ convention. Considering the value = 1 (true):

| format           | presented value |
| ---------------- | --------------- |
| off\|on          | on              |
| stopped\|running | running         |

### Color Tab

**Purpose**: change the fill/stroke color of objects according to limits for the value of points. It is also possible to change attributes, trigger SMIL animations, load images and run small scripts.

**Available for**: all SVG drawing object types (not available for groups).

Each line in the list of limits contains the following fields:

- **Tag**: tag identifier.
- **Limit**: value limit, the color defined in the same row will be used for values equal to or greater than this limit.
- **Color Name/Code**: desired color (SVG named color or #RRGGBB value).

The last true condition of the list will be effective and the others are ignored.

The field “Limit” can have also some special coded values:

- 'a' - for alarmed value (the point has a not yet acknowledged alarm state)
- 'f' - for a failed (invalid quality) value

For digital (boolean) points the following special values for conditions apply:

- 0 – invalid state
- 1 – false (off) state
- 2 – true (on) state
- 3 - transit state
- 130 – invalid state and bad quality
- 129 – false (off) state and bad quality
- 130 – true (on) state and bad quality
- 131 - transit state and bad quality

The colors are the SVG colors (named or #RRGGBB value). “none” is the transparent color.

A single color value will be used as fill and stroke colors. To specify different fill and stroke separate 2 color values by a “\|” (pipe) character. Example: “red\|green” = red for fill and green for stroke.

A void fill color like in “\|yellow” affects only the stroke while keeping the fill unaltered.
A void stroke color like in “black\|” affects only the fill while keeping the stroke unaltered (recommended for text objects).

To interpolate colors between 2 values, use @color in the color field (fill or stroke) in the final line. Example: To make fill colors that varies continuously between white and red proportionally to values between 0 and 10 for the tag “TAG1”.

| Tag  | Limit | Color Name/Code |
| ---- | ----- | --------------- |
| TAG1 | 0     | white\|         |
| TAG1 | 10    | @red\|          |

In the field “Color Name/Code” it's possible to change a SVG attribute instead of the color with the “attrib:” prefix. There must be a space after “attrib:”.

Example of changing attributes

| Tag  | Limit | Color Name/Code                                   |
| ---- | ----- | ------------------------------------------------- |
| TAG1 | 0     | attrib: opacity=0.5                               |
| TAG1 | 10    | attrib: opacity=1.0                               |
| TAG1 | f     | attrib: style=fill:red;text-decoration:underline; |

In the field “Color Name/Code” it's also possible to run a Javascript short script with the “script:” option. There must be a space after “script:”.

The function _$W.Animate_ can be used to animate objects with SMIL (SMIL is not implement in old IE/Edge browsers). The first parameter is the object to be animated (“thisobj” represents the current object); the second is the animation type ('animate', 'set', 'animateTransform', 'animateColor' or 'animateMotion'); the third is the animation options.

    Examples (Color Name/Code field):

    script: $W.Animate( thisobj, 'animate', {'attributeName': 'x', 'from': 0, 'to': 10, 'fill': 'freeze', 'repeatCount': 5, 'dur': 5 } ); // animates on axis x, from 0 to 10 seconds, during 5 seconds, repeats 5 times.

    script: $W.Animate( thisobj, 'animate', {'attributeName': 'width', 'from': 45, 'to': 55, 'repeatCount':5,'dur': 1 } ); // animates width between 45 and 55, 5 times in 1 second.

    script: $W.Animate( thisobj, 'animate', {'attributeName': 'width', 'values': '45;55;45', 'repeatCount':5,'dur': 1 } ); // animates width for the values 45, 55 and  45, 5 times in 1 second.

It's recommended to use _$W.RemoveAnimate(thisobj)_ before creating a new animation to avoid cumulative animations.

See SVG attributes animation documentation, in: http://www.w3.org/TR/SVG/animate.html

For image objects: to load and change images dynamically use the “$W.LoadImage” function as this

    script: $W.LoadImage(thisobj, 'clipart/modem.png');

Special color shortcuts can be changed in _src/htdocs/conf/config_viewers.js_. This file can be used to theme whole drawings provided the codes below are used instead of direct color names.

- "-clr-bgd" – shortcut for the background color (ScreenViewer_Background);
- "-clr-tbr" – shortcut for the toolbar color (ScreenViewer_ToolbarColor);
- "-clr-01" – first user defined shortcut (ScreenViewer_ColorTable[1]);
- "-clr-02" – second user defined shortcut (ScreenViewer_ColorTable[2]);
- …
- … up to 99 user defined color shortcuts.

### Bar Tab

**Purpose**: change the height of a rectangle according to a value.

**Available for**: only for SVG rectangles.

In the “Tag” field, put the desired tag name. The fields “Min” and “Max” represents the expected range of values. The height of the object will be 100% of its original size when the value of the point is equal to the “Max” value and 0% when equal to the value defined in the “Min” field.

Changes in different directions can be obtained by just rotating the object.

### Opacity Tab

**Purpose**: change the opacity (opposite of transparency) of SVG objects according to a value.

**Available for**: all types SVG objects including groups.

In the “Tag” field, put the desired tag name. The fields “Min” and “Max” represent the expected spread of point values. The opacity of the object will be 100% (totally solid) when the value of the point is equal to the Max value and 0% (totally transparent) when equal to the value defined in the Min field.

For digital points consider the value 0 for the ON (true) state and 1 for the OFF (false) state. For Min=0 and Max=1 the object will be solid for the OFF (false) state and will disappear for the ON (true) state. With Min=1 and Max=0 the reverse effect will be obtained.

### Rotate Tab

**Purpose**: rotate the object.

**Available for**: all types of SVG objects including groups.

In the “Tag” field, put the desired tag name. Fill the fields “Max” and “Min” according to the value range of point. When the point reaches the value of “Max” the object will rotate 360 degrees. The object will not rotate when the point has the value of “Min”. The rotation is clockwise when Max > Min, to invert the rotation direction, let Min be greater than Max.

To adjust the center of rotation point of the object, click the object twice until shown the rotation guides (a cross mark is the center of rotation), then press shift and drag the cross at the center of the object.

### Tooltips Tab

**Purpose**: show text when the mouse cursor is over a object.

**Available for**: all types of SVG objects including groups.

The fields “Line 1” to “Line 5” can be filled with the lines of text to be presented. “Size” and “Style” fields are ignored.

The tooltips can contain Javascript code between “!EVAL” and “!END” marks. Use “$V('TAG')” to obtain point values inside the Javascript expression. The expression will be evaluated and the resulting value of it will be shown. What is out of the “!EVAL” and “!END” marks will be presented as text. Indirect variables can be used in the form “$V(%n)” to obtain the point value of a tag defined as variable at the higher level group.

Example: Consider a tag “TAG1” with a value of 22.1 and a point “TAG2” with a value of 10.5.

    Line 1: TAG1+TAG2 = !EVAL $V('TAG1') + $V('TAG2') !END MW

This will present this text (when mouse over): “TAG1+TAG2 = 32.6 MW”.

### Slider Tab

**Purpose**: move the object in a straight line.

**Available for**: all types of SVG objects including groups.

In the “Tag” field, put the desired tag name. The fields “Max” and “Min” must be filled with the desired range of variation for the point.

The SVG object must be cloned (Edit \| Clone \| Create Clone or ALT+D). The original object defines the initial position (this position will be reached when the value is equal to “Min”). The clone object must be positioned at the desired final position (the position to be reached when the value is equal to “Max”).

Movement in the reverse direction can be obtained by switching the values of “Min” and “Max”.

### Zoom Tab

**Purpose**: define a zoom region that is extended to the full viewer when clicked.

**Available for**: all types of SVG objects, except text.

The object must be placed on the top of other objects and have an opacity greater than zero (like “0.1”).

### Script Tab

**Purpose**: associate Javascript code to an event and create charts using the Vega specification.

**Available for**: all types of SVG objects.

Available scriptable events:

- **mouseup**: release the mouse button.
- **mousedown**: mouse click.
- **mouseover**: mouse cursor entering the object.
- **mousemove**: mouse cursor moving over the object.
- **mouseout**: mouse cursor leaving the object.
- **exec_once**: execute a script one time only after the screen is loaded and parsed.
- **exec_on_update**: execute a script every time data is updated.

Use “$V('TAG')” to obtain point values inside the script.

The function $W.Animate and thisobj can be used to animate objects in scripts, example

    var obj = thisobj; // get the current object (the object that hosts the script)

    // Use a call like below to get references to other objects from the SVG file by the id property
    // var obj = SVGDoc.getElementById("rect1");

    $W.RemoveAnimate(obj); // remove previous animations
    // animate on axis x
    $W.Animate(obj, "animate", {"attributeName": "x", "from": 208 ,"to": 300, "repeatCount": 5, "dur": 5});
    // animate on axis y
    $W.Animate(obj, "animate", {"attributeName": "y", "from": -301 ,"to":-400, "repeatCount": 5, "dur": 5});

It's recommended to use $W.RemoveAnimate(thisobj) before creating a new animation to avoid cumulative animations.
Other useful function can toggle the visibility of an object and also apply a translation to it:
$W.ShowHideTranslate( 'id_of_object', x, y );

The function $W.makeDraggable(obj) can be used to make an object draggable by the mouse.

Vega specification markup options:

- **vega**: old style Vega 1/2 specification. In the first line of the script must be written the tag list comma separated. In the next line either a URL to a specification or the specification itself beginning with a “{” char. DEPRECATED, use vega4!
- **vega4**: new style Vega 3/4/5 specification. In the first line of the script must be written the tag list comma separated. In the next line either a URL to a specification or the specification itself beginning with a “{” char.
- **vega-lite**: vega-lite specification. In the first line of the script must be written the tag list comma separated. In the next line either a URL to a specification or the specification itself beginning with a “{” char.
- **vega-json**: old style Vega 1/2 specification with no tags associated. In the first line of the script must be put a URL to a specification or the specification itself beginning with a “{” char. In the data section of the specification define “update_period“ in seconds for the periodic update of the data. DEPRECATED, use vega4-json!
- **vega4-json**: new style Vega 3/4/5 specification with no tags associated. In the first line of the script must be put a URL to a specification or the specification itself beginning with a “{” char. In the data section of the specification define “update_period“ in seconds for the periodic update of the data.

See Vega project site for tools and documentation of syntax: [https://vega.github.io/vega/docs/](https://vega.github.io/vega/docs/).

In the Vega file (“data” / “values” section), use the following markup to refer to the tag list:

    “PNT#1” to retrieve the current value of the first tag in the tag list
    “TAG#1” to retrieve the first tag in the tag list
    “LMI#1” to retrieve the inferior limit of the fist point in the point list
    “LMS#1” to retrieve the superior limit of the fist point in the point list
    “FLG#1” to retrieve the qualifier flags of the first tag in the tag list
    “FLR#1” to retrieve the failure of the first tag in the tag list
    “SUB#1” to retrieve the group1 name (location/station name) of the fist point in the point list
    “BAY#1” to retrieve the group2 name (bay/area name) of the fist point in the point list
    “DCR#1” to retrieve the description of the fist point in the point list
    “HIS#1” to retrieve the historical curve of the first tag in the tag list

### Text Tab

**Purpose**: display predefined texts associated with ranges of values.

**Available for**: text objects.

The “Tag” field must be filled with the desired tag name.

The list of _Tag Values_ and associated _Tag Texts_ should be created with an ascending order of value. The value of the point will be tested against the list of _Tag Values_ to be greater than or equal to each of it. The last true condition will cause the associated text to be presented.

Use a Tag Value of “f” to test for invalid values and “a” for alarmed.

For digital points consider the values (0-3,128-131) as shown previously for the _Color_ tab.

### Faceplate Tab

**Purpose**: this powerful concept allows to replicate groups of animated objects associating each replica to a different tag (or to a different set of tags).

**Available for**: groups of objects.

A model group of objects can be created associating all the animations of an object(s) to an indirection like “%n” (use “%m”, “%p”, etc. to use more tag indirections in the same model). Next, group all related objects that will compose the model. Then use the Faceplate attribute to resolve the indirections in the object with the list of Variables and Values. Variable is the indirection variable like “n” (here you must not put a “%”, just the character of the variable) and Value is the tag associated with it. So, all animations in the grouped objects will be related to the values of the resolved tag. Copy and paste the grouped object and change the tags of the variable(s) to obtain new objects with the same animations linked to other tags.

See examples of this concept in the demo displays included with the installation or in the Github repo.

### Popup Tab

**Purpose**: control the action when the object is clicked.

**Available for**: all types of SVG objects including groups.

The field “Source” can be:

- A tag – set the tag to opened in the point info dialog when the object is clicked.
- “block” – block any action when object clicked.
- “notrace” – allow point info dialog when object is clicked but do not highlight the object when accessed.
- “preview:URL”: presents a preview of the URL when the mouse is over the object. The width and height parameters define the preview window size.

### Open Tab

**Purpose**: plot trends, open new pages, preview pages.

**Available for**: all types of SVG objects including groups.

- For field “Source Type” = URL

  - Field Source = "new:URL" - link to open new page with URL contents when clicked.
  - Field Source = "preview:URL" - show preview box with URL contents (when mouse over the object).
  - Field Source = "filename.svg" - link to and show preview box (mouse over) with display file contents.

  Select field "Dest.Type" = "New exclusive window" to set width and height of the preview window. Fields "X-position" and "Y-position" are ignored (new windows or preview windows are auto positioned).

  If a preview window exceeds the available space inside the Display Viewer window it will not be shown.

- For rectangle objects and “Source Type” = “Tag”: draw a line with a trend plot inside the rectangle with the rectangle's stroke width and color.
  Parameters:
  - Field Source = desired tag to plot.
  - Field X-position = must be 0.
  - Field Y-position = vertical offset value.
  - Field Width = horizontal (X) range in seconds, plot time window continuously moved, current value plotted to the right edge. If negative, plot from the last round time (for the range), current value plotted from left to right, restart plot when current date is bigger than final time for the last range (can be used to plot values ahead of time, like forecasts).
  - Field Height = vertical (Y) range for point values.

### Set Tab

**Purpose**: configure the special functions.
**Available for**: all types of SVG objects.

Functions available:

- **#exec** or **#exec_once** in the field “Tag” - execute once the script entered in the field “Source”.
- **#exec_on_update** in the field “Tag” - execute the script entered in the field “Source” each time the screen is refreshed with new data.
- **#copy_xsac_from** in the field “Tag” - copy the XSAC attributes from another model object(s) to the current object. Use the field “Source” to indicate the ID of the model object. Multiple ID's of model objects can be entered in the field “Source” separating them with commas. The other fields are ignored. This can be used to create models of actions that control the behavior of many other derived objects changing just the model object. This can be combined with the Faceplate attribute to replicate modeled objects.
- **#set_filter** in the field “Tag” – define a filter (by the point ID) for the data presented in the Alarm Box. Use the field "Source" to enter the text of the filter. The other fields are ignored.
- **#arc** in the field Tag – draw a doughnut chart. The tag must be in the “Source” field. In the “Prompt” field there must be set three parameters separated by commas: the minimum value (normally zero), the maximum value (for a 360-degree arc) and the inner circle radius.
- **#camera** in the field Tag – open a camera view (inside a foreignObject/iframe). The field “Source” must contain the camera name (e.g. "CAM001"). The field “Prompt” can be used to define the iframe properties (e.g. '"width=500 height=500 style="transform: scale(0.5);transform-origin: 0 0;" frameborder="0" scrolling="no"').
- **#foreign_object** in the field Tag – open a generic URL inside a foreignObject/iframe. The field “Source” must contain the URL (e.g. "events.html"). The field “Prompt” can be used to define the iframe properties (e.g. "width=100% height=100%").

- **#vega4**, **#vega4-json** or **#vega-lite** in the field “Tag” - define a Vega (version 3/4/5) or VegaLite chart. List the tags in the “Source” field separated by commas. You can set the number of minutes to retrieve for historical data putting the pipe character and a number after the point list in the “Source” field (e.g.: “38038\|15”). The field Prompt must contain the Vega chart specification (JSON code that must begin with a ‘”{” ) or a URL link to a file (e.g. “http://site.com/charts/stacked.json”).

See the Vega project site for tools and documentation of syntax: [https://vega.github.io/vega/docs/](https://vega.github.io/vega/docs/).

In the Vega file (“data” / “values” section), use the markup below to refer to the tag list (from the “Source” field).

    “PNT#1” to retrieve the current value of the first tag in the tag list
    “TAG#1” to retrieve the first tag in the tag list
    “LMI#1” to retrieve the inferior limit of the fist point in the point list
    “LMS#1” to retrieve the superior limit of the fist point in the point list
    “FLG#1” to retrieve the qualifier flags of the first tag in the tag list
    “FLR#1” to retrieve the failure of the first tag in the tag list
    “SUB#1” to retrieve the group1 name (location/station name) of the fist point in the point list
    “BAY#1” to retrieve the group2 name (bay/area name) of the fist point in the point list
    “DCR#1” to retrieve the description of the fist point in the point list
    “HIS#1” to retrieve the historical curve of the first tag in the tag list

### Special Codes

Obs.: Special codes to obtain other point attributes instead of the tag value in SCADA animations, "xxx" represents the tag name.

- !ALMxxx = returns “1” for the unacknowledged alarm or abnormal state and “0” for the acknowledged and normal state.
- !ALRxxx = returns “1” for the unacknowledged alarm and “0” for acknowledged or not alarmed state.
- !TMPxxx = returns the time of the last alarm for the point.
- !SLIMxxx = returns the upper analog limit.
- !ILIMxxx = returns the lower analog limit.
- !TAGxxx = returns the point tag name.
- !DCRxxx = returns the point description.
- !STONxxx = returns the text for the ON state of digital points.
- !STOFFxxx = returns the text for the OFF state of digital points.
- !STVALxxx = returns the text for current state of digital points.
- !EVAL expression = evaluates a Javascript expression, use $V("xxx") to obtain point values in the expression.

### Debugging Scripts is Displays

To debug scripts in a SVG display file, do the following:

- include the keyword “debugger;” at the beginning of the script. This will work as a breakpoint.
- open the display in the Display Viewer.
- press F12 to open the browser Developer Tools and then F5 to reload.
- The Chromium browser will stop execution when found the introduced breakpoint. Use the execution control keys F10, F11, F9, F8 to forward execution.
