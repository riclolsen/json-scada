---
name: svg-scada
description: Generate, structure, and markup scalable vector graphics (SVG) files that natively interface with SCADA markups by embedding strict, HTML-escaped JSON configurations into the `inkscape:label` attribute of SVG elements. Dedicated to the SCADAvis.io / OSHMI / JSON-SCADA projects and integration specialist. Use when user asks to edit or create SVG files with SCADA animations or markup (SVG or SCADA or SAGE or XSAC or Inkscape or SVG-Edit or SVG-SCADA or JSON-SCADA or SCADAvis or OSHMI).
---

# SVG SCADA Generation Agent Skills

## Overview

This agent is designed to generate, structure, and markup scalable vector graphics (SVG) files that natively interface with SCADA markups by embedding strict, HTML-escaped JSON configurations into the `inkscape:label` attribute of SVG elements. It is dedicated to the SCADAvis.io / OSHMI / JSON-SCADA projects.

## 📐 Core SVG & JSON Serialization Rules

1. **Namespace Required:** The `<svg>` root element **must** include the Inkscape namespace: `xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape"`.
2. **JSON Object Structure:** The SCADA parameters must be a flat JSON dictionary. The JSON content of SCADA markup must be put in the " inkscape:label" attribute of the SVG element affected.
3. **The "attr" Key:** The selected animation "Tab" (action type) must be stored in the `"attr"` key (e.g., `"attr":"set"`, `"attr":"get"`, `"attr":"bar"`).
4. **Lowercase Keys:** All JSON keys correspond to the dialog fields and MUST be lowercase (e.g., `"tag"`, `"src"`, `"prompt"`, `"format"`, `"min"`, `"max"`).
5. **XML-Entity Escaping:** The JSON string MUST be HTML-escaped so that standard double-quotes inside the JSON become `&quot;`. The attribute itself is wrapped in standard double quotes.
   - _Correct Example:_ `inkscape:label="{&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;TAG1&quot;,&quot;format&quot;:&quot;%5.2f&quot;}"`

---

## ⚙️ SCADA Animations & Attribute Mapping

Use the following JSON dictionary structures embedded as HTML-escaped strings for different SCADA behaviors.
Multiple behaviors can be combined by embedding multiple JSON objects in the same `inkscape:label` attribute, as a JSON array but without the surrounding square brackets `[]` (e.g., `{"attr":"get", "tag":"TAG_NAME"},{"attr":"color", "tag":"TAG_NAME", "limit1":"1", "color1":"red|"}`).

### 1. Get (Text Formatting)

- Targets `<text>` SVG elements.
- The `<text>` element should contain a `<tspan>` child element where the format string is placed (e.g., `%5.2f`, `.3s`, `off|on|failed`).
- **Fields:** `"attr":"get"`, `"tag"`, `align` (e.g., `"align":"Right"`), `type` (e.g., `"type":"Good"`). All fields are required to ensure proper parsing and functionality.
- **JSON:** `{"attr":"get", "tag":"TAG_NAME", "align":"Right", "type":"Good"}`
- **Format Strings should be put in the tspan element:**
  - Printf (`%5.2f`), d3 (`.3s`), Boolean (`off|on|failed`).
  - Flow arrows: append `u^`, `d^`, `r^`, `l^`, `a^` to show direction.
- Example: `<text
   xml:space="preserve"
   style="font-style:normal;font-variant:normal;font-weight:normal;font-stretch:normal;line-height:0%;font-family:'Microsoft Sans Serif';-inkscape-font-specification:'Microsoft Sans Serif';text-align:end;letter-spacing:0px;word-spacing:0px;writing-mode:lr-tb;text-anchor:end;fill:#6a43e5;fill-opacity:1;stroke:none"
   x="-310"
   y="-232.63782"
   id="mw_model"
   inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;%n&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
     sodipodi:role="line"
     id="tspan4770"
     x="-310"
     y="-232.63782"
     style="font-size:22px;line-height:1.25">%5.1f</tspan></text>
`.

### 2. Color (Fill, Stroke, & Attributes)

- **Target:** SVG drawing objects (Exclude `<g>`).
- **Fields:** `"attr":"color"`, `"tag"`, `"list"` as an array of `{"data":"VALUE_LIMIT"}`, `{"param":"COLOR_CODE"}`, `{"tag":"TAG_NAME"}`.
- **Color Syntax:** `"fill|stroke"`, e.g., `"red|green"`, `"|yellow"` (stroke only), `"black|"` (fill only).
- **Limit Constants:** `"a"` (alarm), `"f"` (failed), `"1"` (off), `"2"` (on).
- **List order:** entries are tested in order and the **last** match wins (a digital matches `"1"` when off or on, `"2"` only when on; an analog matches every limit at or below its value), so list digital states as `"1"`, `"2"`, then `"a"`/`"f"`, and analog limits in ascending order.
- **JSON:** `{"attr":"color", "tag":"TAG_NAME", "list":[{"data":"1", "param":"red|", "tag":"%n"}, {"data":"2", "param":"green|", "tag":"%n"}, {"data":"f", "param":"gray|", "tag":"%n"}]}`. All fields are required to ensure proper parsing and functionality.
- **Attribute Hack:** Use the prefix `"attrib:"` followed by SVG properties for the "data" field of "list" arrray (e.g., `{"data":"attrib: style=fill:red;text-decoration:underline;"}`) to manipulate SVG properties instead of just colors.
- Example: `inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;list&quot;:[{&quot;data&quot;:&quot;-99999&quot;,&quot;param&quot;:&quot;-cor-11|&quot;,&quot;tag&quot;:&quot;%n&quot;},{&quot;data&quot;:&quot;-500&quot;,&quot;param&quot;:&quot;-cor-20|&quot;,&quot;tag&quot;:&quot;%n&quot;},{&quot;data&quot;:&quot;500&quot;,&quot;param&quot;:&quot;-cor-11|&quot;,&quot;tag&quot;:&quot;%n&quot;},{&quot;data&quot;:&quot;c&quot;,&quot;param&quot;:&quot;-cor-41|&quot;,&quot;tag&quot;:&quot;%n&quot;},{&quot;data&quot;:&quot;n&quot;,&quot;param&quot;:&quot;-cor-42|&quot;,&quot;tag&quot;:&quot;%n&quot;},{&quot;data&quot;:&quot;f&quot;,&quot;param&quot;:&quot;-cor-12|&quot;,&quot;tag&quot;:&quot;%n&quot;}]}"`.

### 3. Bar, Opacity, Rotate, Slider

- **Bar:** `{"attr":"bar", "tag":"TAG", "min":0, "max":100}` (Targets `<rect>` SVG elements. Max matches the the variable value proportionally to the height of the rectangle). All fields are required to ensure proper parsing and functionality.
- **Rotate:** `{"attr":"rotate", "tag":"TAG", "min":0, "max":100}` (Rotates 360° from Min to Max). Targets any SVG element. All fields are required to ensure proper parsing and functionality.
- **Slider:** `{"attr":"slider","max":100,"min":0,"readonly":0,"tag":"TAG1"}` (Linearly transitions from the original object's position to a cloned object's (`<use>` SVG element) position). Targets any SVG element. All fields are required to ensure proper parsing and functionality.

### 4. Tooltips

- Targets SVG element. The tooltip can display up to 5 lines of text when the mouse hovers over the element.
- **JSON:** `{"attr":"tooltips","param":["text1","text2","text3","text4","!EVAL $V('TAG') !END"],"size":12,"style":""}`. All fields are required to ensure proper parsing and functionality.

### 5. Popup & Open

- Targets SVG element.
- **Popup:** `{"attr":"popup","height":400,"src":"TAG_OR_ACTION","width":500,"x":100,"y":100}`
  _Popup Actions: `block` (block the popup from opening), `notrace` ( allow point info dialog when object is clicked but do not highlight the object when accessed), `preview:URL` (Show a preview of the URL in a window when mouse hovers over the element)._
- **Open (URL):** `{"attr":"open","height":400,"istag":0,"src":"http://localhost:8080/index.html","type":"_self|_blank|_shared","width":500,"x":100,"y":100}`
- **Open (Trends):** `{"attr":"open","height":400,"istag":1,"src":"TAG_NAME","type":"_self|_blank|_shared","width":500,"x":100,"y":100}`\_(Draws a line-trend inside a`<rect>`)\_.
- All fields are required to ensure proper parsing and functionality.

### 6. Faceplate (Indirect Variables)

- Targets SVG group `<g>` elements.
- **JSON:** `{"attr":"clone","map":["%n=TAG1","%m=TAG2"]}`
- **Inner Elements:** Elements inside the group reference the variable using `%` (e.g., `"TAG1" as "%n"` and `"TAG2" as "%m"`).
- All fields are required to ensure proper parsing and functionality.

### 7. Set (Macros & Functions)

- **Target:** Any object.
- **Fields:** `{"align":"Left|Right","attr":"set","prompt":"PROMPT_TEXT","src":"SOURCE_TEXT","tag":"TAG1","type":"Data|Variable"}`.
- **Use Cases:**
  - **Arc/Doughnut:** `{"attr":"set", "tag":"#arc", "src":"TAG1", "prompt":"0,100,75", "align":"Right", "type":"Data"}` _(Prompt = Min,Max,InnerRadius)_.
  - **Execute Script:** `{"attr":"set", "tag":"#exec_once", "src":"console.log('loaded');", "align":"Right", "type":"Data"}`
  - **Clone Properties:** `{"attr":"set", "tag":"#copy_xsac_from", "src":"MASTER_ELEMENT_ID", "align":"Right", "type":"Data"}`
  - **Vega Charts:** `{"attr":"set", "tag":"#vega4", "src":"TAG1,TAG2", "prompt":"URL_OR_JSON", "align":"Right", "type":"Data"}`
  - **ONVIF Camera:** `{"attr":"set", "tag":"#camera", "src":"CAM_NAME", "prompt":"width=500 height=500", "align":"Right", "type":"Data"}`
- All fields are required to ensure proper parsing and functionality.

### 8. Script

- **Purpose:** Associate Javascript code to an event and create charts using the Vega specification.
- **Target:** Any object.
- **Fields:** `{"attr":"script","list":[{"evt":"mouseup","param":"SCRIPT_CONTENT"},{"evt":"mousedown","param":"SCRIPT_CONTENT"},{"evt":"mouseover","param":"SCRIPT_CONTENT"},{"evt":"mouseout","param":"SCRIPT_CONTENT"},{"evt":"mousemove","param":"SCRIPT_CONTENT"},{"evt":"keydown","keys":"","param":"SCRIPT_CONTENT"},{"evt":"exec_once","keys":"","param":"SCRIPT_CONTENT"},{"evt":"exec_on_update","keys":"","param":"SCRIPT_CONTENT"},{"evt":"vega-lite","keys":"","param":"SCRIPT_CONTENT"},{"evt":"vega","keys":"","param":"SCRIPT_CONTENT"},{"evt":"vega-json","keys":"","param":"SCRIPT_CONTENT"},{"evt":"vega4","keys":"","param":"SCRIPT_CONTENT"},{"evt":"vega4-json","keys":"","param":"SCRIPT_CONTENT"}]}`.
- **Use Cases:**
  Available scriptable events:

  mouseup: release the mouse button.
  mousedown: mouse click.
  mouseover: mouse cursor entering the object.
  mousemove: mouse cursor moving over the object.
  mouseout: mouse cursor leaving the object.
  exec_once: execute a script one time only after the screen is loaded and parsed.
  exec_on_update: execute a script every time data is updated.

  Use “$V('TAG')” to obtain point values inside the script, and “$J('TAG', '$.path')” for members of a point's structured value (see section 10).

  The function $W.Animate and thisobj can be used to animate objects in scripts, example

  var obj = thisobj; // get the current object (the object that hosts the script)

  // Use a call like below to get references to other objects from the SVG file by the id property
  // var obj = SVGDoc.getElementById("rect1");

  $W.RemoveAnimate(obj); // remove previous animations
  // animate on axis x
  $W.Animate(obj, "animate", {"attributeName": "x", "from": 208 ,"to": 300, "repeatCount": 5, "dur": 5});
  // animate on axis y
  $W.Animate(obj, "animate", {"attributeName": "y", "from": -301 ,"to":-400, "repeatCount": 5, "dur": 5});

  Vega specification markup options:

      vega: old style Vega 1/2 specification. In the first line of the script must be written the tag list comma separated. In the next line either a URL to a specification or the specification itself beginning with a “{” char. DEPRECATED, use vega4! Example:
      `{"attr":"script","list":[{"evt":"vega","param":"TAG1,TAG2,TAG3,TAG4,TAG5,TAG6,TAG7\n{\n  \"width\": 200,\n  \"height\": 200,\n  \"data\": [\n    {\n      \"name\": \"correntes\",\n      \"values\": [\n        {\"x\": 1,  \"y\": \"PNT#1\", \"bay\" : \"BAY#1\"},\n        {\"x\": 2,  \"y\": \"PNT#2\", \"bay\" : \"BAY#2\"},\n        {\"x\": 3,  \"y\": \"PNT#3\", \"bay\" : \"BAY#3\"},\n        {\"x\": 4,  \"y\": \"PNT#4\", \"bay\" : \"BAY#4\"},\n        {\"x\": 5,  \"y\": \"PNT#5\", \"bay\" : \"BAY#5\"},\n        {\"x\": 6,  \"y\": \"PNT#6\", \"bay\" : \"BAY#6\"},\n        {\"x\": 7,  \"y\": \"PNT#7\", \"bay\" : \"BAY#7\"},\n        {\"x\": 8,  \"y\": \"PNT#8\", \"bay\" : \"BAY#8\"}\n      ],\n      \"transform\": [{\"type\": \"pie\", \"field\": \"y\", \"sort\": \"true\"}]\n    } ],\n  \"scales\": [\n    {\n      \"name\": \"r\",\n      \"type\": \"linear\",\n      \"domain\": {\"data\": \"correntes\", \"field\": \"y\"},\n      \"range\": [20, 100]\n    },\n    {\n      \"name\": \"c\",\n      \"type\": \"ordinal\",\n      \"range\": [\"#74add1\", \"#4575b4\", \"#dcfab9\", \"#d9a6ff\", \"#e0f3f8\", \"#ffffbf\", \"#ffc794\", \"#fee090\"]\n    }\n  ],  \n  \"marks\": [\n     {\n      \"type\": \"arc\",\n      \"from\": {\"data\": \"correntes\"},\n      \"properties\": {\n        \"enter\": {\n          \"x\": {\"field\": {\"group\": \"width\"}, \"mult\": 0.5},\n          \"y\": {\"field\": {\"group\": \"height\"}, \"mult\": 0.5},\n          \"startAngle\": {\"field\": \"layout_start\"},\n          \"endAngle\": {\"field\": \"layout_end\"},\n          \"innerRadius\": {\"value\": 20},\n          \"outerRadius\": {\"value\": 60},\n          \"fill\": {\"scale\": \"c\", \"field\": \"x\"}\n        },\n        \"update\": {\n          \n        },\n        \"hover\": {\n          \"fill\": {\"value\": \"pink\"}\n        }\n      }\n    },\n    {\n      \"type\": \"text\",\n      \"from\": {\"data\": \"correntes\"},\n      \"properties\": {\n        \"enter\": {\n          \"x\": {\"field\": {\"group\": \"width\"}, \"mult\": 0.5},\n          \"y\": {\"field\": {\"group\": \"height\"}, \"mult\": 0.5},\n          \"radius\": {\"value\": \"45\"},\n          \"theta\": {\"field\": \"layout_mid\"},\n          \"align\": {\"value\": \"center\"},\n          \"baseline\": {\"value\": \"middle\"},\n          \"fill\": {\"value\": \"#002d75\"},\n          \"font\": {\"value\": \"open sans\"},\n          \"fontSize\": {\"value\": 14},\n          \"fontStyle\": {\"value\": \"oblique\"},\n          \"text\": { \"template\": \"{{datum.y|number:'.0f'}}\" }\n        }\n      }\n    },\n    {\n      \"type\": \"text\",\n      \"from\": {\"data\": \"correntes\"},\n      \"properties\": {\n        \"enter\": {\n          \"x\": {\"field\": {\"group\": \"width\"}, \"mult\": 0.5},\n          \"y\": {\"field\": {\"group\": \"height\"}, \"mult\": 0.5},\n          \"radius\": {\"value\": \"65\"},\n          \"theta\": {\"field\": \"layout_mid\"},\n          \"align\": {\"value\": \"center\"},\n          \"baseline\": {\"value\": \"middle\"},\n          \"fill\": {\"value\": \"#0088ee\"},\n          \"font\": {\"value\": \"open sans\"},\n          \"fontSize\": {\"value\": 12},\n          \"fontWeight\": {\"value\": 300},\n          \"text\": { \"template\": \"{{datum.bay|left:4}}\" }\n        }\n      }\n    }\n  ]\n}"}]}`

      vega4: new style Vega 3/4/5 specification. In the first line of the script must be written the tag list comma separated. In the next line either a URL to a specification or the specification itself beginning with a “{” char.
      vega-lite: vega-lite specification. In the first line of the script must be written the tag list comma separated. In the next line either a URL to a specification or the specification itself beginning with a “{” char.
      vega-json: old style Vega 1/2 specification with no tags associated. In the first line of the script must be put a URL to a specification or the specification itself beginning with a “{” char. In the data section of the specification define “update_period“ in seconds for the periodic update of the data. DEPRECATED, use vega4-json!
      vega4-json: new style Vega 3/4/5 specification with no tags associated. In the first line of the script must be put a URL to a specification or the specification itself beginning with a “{” char. In the data section of the specification define “update_period“ in seconds for the periodic update of the data.
      See Vega project site for tools and documentation of syntax: https://vega.github.io/vega/docs/.

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

- All fields are required to ensure proper parsing and functionality.

### 9. Communication Status Monitoring

The data processor keeps system tags with the health of every protocol driver instance and connection (names and points in `docs/schema.md`, "Communication status tags"). Bind them like any other tag; characters outside `A-Z a-z 0-9 _ -` in a connection or driver name become `_` (connection `KAW 2` → `_System.Conn.KAW_2.Status`).

- **Connection LED** (digital, on = Online, off = Offline, failed = unknown): `{"attr":"color","tag":"_System.Conn.KAW2.Status","list":[{"data":"1","param":"red|","tag":"%n"},{"data":"2","param":"lime|","tag":"%n"},{"data":"f","param":"gray|","tag":"%n"}]}`
- **Driver running** (digital, on = Running): same color markup on `_System.Driver.DNP3.1.Alive`.
- **Link state** (digital, failed when the driver does not report it): `_System.Conn.KAW2.LinkUp`, format `Link Down|Link Up|n/a`.
- **Health % of valid tags** (analog): `{"attr":"get","tag":"_System.Conn.KAW2.Health","align":"Right","type":"Good"}` with tspan `%3.0f` (put the `%` unit in a separate static text), or a bar `{"attr":"bar","tag":"_System.Conn.KAW2.Health","min":0,"max":100}`.
- **Overview badge** (analog): `_System.Comm.ConnectionsDown` with a color list `[{"data":"0","param":"lime|","tag":"%n"},{"data":"1","param":"red|","tag":"%n"}]`.
- **One faceplate per connection:** build the row once in a `<g>` with `{"attr":"clone","map":["%n=_System.Conn.KAW2.Status","%m=_System.Conn.KAW2.Health"]}` and copy it, changing only the map.
- `ActiveNode` and `Version` are string tags: the display `get` shows numbers, so show them in the point info popup or the tabular viewer, not as display text.
- Bind only connections and driver instances that exist: the tags are created about 5 s after the configuration entry and removed with it, so a display bound to a missing one renders it as failed.

### 10. Structured Values (valueJson)

Drivers such as MQTT/Sparkplug B, OPC UA, Node-RED and N8N store a tag's structured value (object or array) as JSON text in its `valueJson` field (a `json` or `string` tag without one uses its string value). The integrated Display Viewer (AdminUI) reads members of it; the legacy `display.html` viewer does not support this markup.

- **Tag reference:** write `!JSON <TAG> <PATH>` wherever a tag goes: `get`, `color` list tags, `bar`, `opac`, `rotate`, `slider`, `text`, trend `open` (realtime samples only, no history), `#arc`/`#radar`/Vega tag lists. The tag may also be a point key or a `%n` clone variable. Clicking the object opens the parent tag.
- **Path** (starts with `$`, which alone means the whole value):
  - `.name` member; `['name']` member whose name has `.`, `[`, `]` or spaces
  - `[0]` array element, `[-1]` the last one
  - `[field=value]` first array element whose `field` equals `value` (quote values with spaces: `[name='Pump 2']`)
  - `.length` size of an array or string
- Use single quotes inside the path: the label JSON already uses double quotes.
- **Value types:** `true`/`false` behave like a digital (on/off: `"1"`/`"2"` colors, `OFF|ON` text); numbers like an analog; strings show with `%s` (or an empty format); objects and arrays show as JSON text. Quality, alarm and annotation come from the tag. A missing member keeps the last value found and shows as failed (`"f"`).
- **Examples** (tag `MQTT.plant` holding `{"pumps":[{"name":"P1","speed":1450.5,"on":true}],"motor.temp":61.2}`):
  - Speed text (tspan `%6.1f`): `{"attr":"get","tag":"!JSON MQTT.plant $.pumps[0].speed","align":"Left","type":"Good"}`
  - Pump LED: `{"attr":"color","tag":"!JSON MQTT.plant $.pumps[name=P1].on","list":[{"data":"1","param":"red|","tag":"!JSON MQTT.plant $.pumps[name=P1].on"},{"data":"2","param":"lime|","tag":"!JSON MQTT.plant $.pumps[name=P1].on"},{"data":"f","param":"gray|","tag":"!JSON MQTT.plant $.pumps[name=P1].on"}]}`
  - Member with a dot in its name: `{"attr":"bar","tag":"!JSON MQTT.plant $['motor.temp']","min":0,"max":100}`
- **Scripts and `!EVAL`:** `$J('TAG')` returns the whole parsed value and `$J('TAG', '$.path')` one member (`undefined` when absent); `$W.getJson` is the same function. Example tooltip line: `"Pumps: !EVAL $J('MQTT.plant', '$.pumps.length') !END"`.

---

## 💻 AI Implementation Example: Accurate Escaped Output

**User Prompt:** Create a digital text display for a pump status and a gauge (using the `#arc` set function), fully bound with JSON-SCADA logic.

**AI Response Output Format:**

```xml
<svg xmlns="http://www.w3.org/2000/svg"
     xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape"
    xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd"
    xmlns="http://www.w3.org/2000/svg"
    xmlns:svg="http://www.w3.org/2000/svg"
    xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
    xmlns:cc="http://creativecommons.org/ns#"
    xmlns:dc="http://purl.org/dc/elements/1.1/"
      width="2400" height="1500" viewBox="0 0 2400 1500">

    <g
     inkscape:label="Layer 1"
     inkscape:groupmode="layer"
     id="layer1">

      <!-- Example 1: Pump Status Digital Display -->
      <g id="pump_status_group">

        <!-- Background changes color based on PUMP_01 state -->
        <!-- JSON: {"attr":"color", "tag":"PUMP_01", "limit1":"1", "color1":"red|", "limit2":"2", "color2":"green|", "limit3":"f", "color3":"gray|"} -->
        <rect id="pump_bg"
              x="100" y="100" width="200" height="50"
              fill="gray" stroke="black" stroke-width="2"
              inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;tag&quot;:&quot;PUMP_01&quot;,&quot;limit1&quot;:&quot;1&quot;,&quot;color1&quot;:&quot;red|&quot;,&quot;limit2&quot;:&quot;2&quot;,&quot;color2&quot;:&quot;green|&quot;,&quot;limit3&quot;:&quot;f&quot;,&quot;color3&quot;:&quot;gray|&quot;}" />

        <!-- Text formats the PUMP_01 state into strings -->
        <!-- JSON: {"attr":"get", "tag":"PUMP_01", "format":"STOPPED|RUNNING|COMM ERROR"} -->
        <text id="pump_text"
              x="200" y="135"
              font-family="Arial" font-size="24" text-anchor="middle" fill="white"
              inkscape:label="{&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;PUMP_01&quot;,&quot;format&quot;:&quot;STOPPED|RUNNING|COMM ERROR&quot;}">
            STOPPED
        </text>
      </g>

      <!-- Example 2: Analog Level using #arc donut macro -->
      <!-- JSON: {"attr":"set", "tag":"#arc", "src":"TANK_LVL", "prompt":"0,100,75"} -->
      <g id="tank_arc_group">
        <path id="tank_level_arc"
              d="M 500,500 A 100,100 0 0,1 600,600"
              fill="blue"
              inkscape:label="{&quot;attr&quot;:&quot;set&quot;,&quot;tag&quot;:&quot;#arc&quot;,&quot;src&quot;:&quot;TANK_LVL&quot;,&quot;prompt&quot;:&quot;0,100,75&quot;}" />
      </g>
    </g>

</svg>
```

## Complete Complex SVG Example

```xml
<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<!-- Created with Inkscape (http://www.inkscape.org/) -->

<svg
   width="1000"
   height="1000"
   viewBox="0 0 264.58333 264.58333"
   version="1.1"
   inkscape:version="1.4.3 (5eeaa1c2f6, 2026-04-11)"
   xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape"
   xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd"
   xmlns="http://www.w3.org/2000/svg"
   xmlns:svg="http://www.w3.org/2000/svg">
  <g
     inkscape:label="Layer 1"
     inkscape:groupmode="layer"
     id="layer1">
    <rect
       style="opacity:0.98;fill:#dde9af;fill-opacity:1;stroke:none;stroke-width:1.68677;stroke-miterlimit:4;stroke-dasharray:none;stroke-dashoffset:0"
       id="rect7554"
       width="156.36104"
       height="6.2012873"
       x="7.2491589"
       y="12.074057" />
    <rect
       style="opacity:0.98;fill:#aa0000;fill-opacity:1;stroke:#ff0000;stroke-width:1.0958;stroke-miterlimit:4;stroke-dasharray:none;stroke-dashoffset:0"
       id="rect27732"
       width="10.179841"
       height="8.9269915"
       x="104.25072"
       y="144.14891" />
    <text
       xml:space="preserve"
       style="font-style:normal;font-variant:normal;font-weight:bold;font-stretch:normal;font-size:5.18951px;line-height:1.25;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Bold';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:start;letter-spacing:0px;word-spacing:0px;writing-mode:lr-tb;text-anchor:start;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.129736"
       x="9.4246464"
       y="24.715755"
       id="text6842-5-0"><tspan
         sodipodi:role="line"
         id="tspan6840-7-0"
         x="9.4246464"
         y="24.715755"
         style="font-style:normal;font-variant:normal;font-weight:bold;font-stretch:normal;font-size:5.18951px;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Bold';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:start;writing-mode:lr-tb;text-anchor:start;stroke-width:0.129736">Get Tab</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="13.312727"
       y="47.067356"
       id="text8053"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051"
         x="13.312727"
         y="47.067356"
         style="fill:#0000ff;stroke-width:0.140579">%f</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="13.312727"
       y="53.489655"
       id="text8053-5"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8"
         x="13.312727"
         y="53.489655"
         style="fill:#0000ff;stroke-width:0.140579">%.1f</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="13.312727"
       y="59.911945"
       id="text8053-5-6"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8-1"
         x="13.312727"
         y="59.911945"
         style="fill:#0000ff;stroke-width:0.140579">%08.2f</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="89.871658"
       y="47.325474"
       id="text8053-5-6-7"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8-1-4"
         x="89.871658"
         y="47.325474"
         style="fill:#0000ff;stroke-width:0.140579">.^20</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="90.162704"
       y="54.561752"
       id="text8053-5-6-7-8"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8-1-4-0"
         x="90.162704"
         y="54.561752"
         style="fill:#0000ff;stroke-width:0.140579">,.2r</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="13.213881"
       y="66.334244"
       id="text8053-5-6-75"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8-1-2"
         x="13.213881"
         y="66.334244"
         style="fill:#0000ff;stroke-width:0.140579">r^%.2f</tspan></text>
    <g
       id="g7562"
       inkscape:label="{&quot;attr&quot;:&quot;tooltips&quot;,&quot;param&quot;:[&quot;Use the printf convention to format numbers and strings.&quot;],&quot;size&quot;:12,&quot;style&quot;:&quot;&quot;}"
       transform="matrix(0.15660616,0,0,0.15660616,6.2852055,8.0136647)">
      <text
         id="text6842-5-0-9"
         y="196.34877"
         x="20.477276"
         style="font-style:normal;font-weight:normal;font-size:33.1369px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.828422"
         xml:space="preserve"><tspan
           style="stroke-width:0.828422"
           y="196.34877"
           x="20.477276"
           id="tspan6840-7-0-3"
           sodipodi:role="line">Printf Convention</tspan></text>
    </g>
    <g
       id="g7566"
       inkscape:label="{&quot;attr&quot;:&quot;tooltips&quot;,&quot;param&quot;:[&quot;The D3 convention is more powerful than the printf convention.&quot;],&quot;size&quot;:12,&quot;style&quot;:&quot;&quot;}"
       transform="matrix(0.15660616,0,0,0.15660616,18.139407,8.0136647)">
      <text
         id="text6842-5-0-9-7"
         y="197.61737"
         x="434.63873"
         style="font-style:normal;font-weight:normal;font-size:33.1369px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.828422"
         xml:space="preserve"><tspan
           style="stroke-width:0.828422"
           y="197.61737"
           x="434.63873"
           id="tspan6840-7-0-3-2"
           sodipodi:role="line">D3 Convention</tspan></text>
    </g>
    <text
       xml:space="preserve"
       style="font-style:normal;font-variant:normal;font-weight:bold;font-stretch:normal;font-size:5.18951px;line-height:1.25;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Bold';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:start;letter-spacing:0px;word-spacing:0px;writing-mode:lr-tb;text-anchor:start;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.129736"
       x="9.4246464"
       y="123.66969"
       id="text6842-5-0-1"><tspan
         sodipodi:role="line"
         id="tspan6840-7-0-8"
         x="9.4246464"
         y="123.66969"
         style="font-style:normal;font-variant:normal;font-weight:bold;font-stretch:normal;font-size:5.18951px;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Bold';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:start;writing-mode:lr-tb;text-anchor:start;stroke-width:0.129736">Color Tab</tspan></text>
    <rect
       style="fill:#ffffff;fill-opacity:1;stroke:#000000;stroke-width:0.626423;stroke-miterlimit:4;stroke-dasharray:none"
       id="rect3847"
       width="10.962432"
       height="10.962432"
       x="15.37694"
       y="145.01048"
       inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;list&quot;:[{&quot;data&quot;:&quot;-999999&quot;,&quot;param&quot;:&quot;none|red&quot;,&quot;tag&quot;:&quot;@1&quot;}]}" />
    <rect
       style="fill:#ffffff;fill-opacity:1;stroke:#000000;stroke-width:0.626423;stroke-miterlimit:4;stroke-dasharray:none"
       id="rect3847-6"
       width="10.962432"
       height="10.962432"
       x="42.939621"
       y="145.01048"
       inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;list&quot;:[{&quot;data&quot;:&quot;-999999&quot;,&quot;param&quot;:&quot;#00ff00|black&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;0&quot;,&quot;param&quot;:&quot;#00ff00|black&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;200&quot;,&quot;param&quot;:&quot;red|black&quot;,&quot;tag&quot;:&quot;@1&quot;}]}" />
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.58978px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0647447"
       x="14.853045"
       y="138.24452"
       id="text6842-5-0-1-3"><tspan
         sodipodi:role="line"
         id="tspan6840-7-0-8-9"
         x="14.853045"
         y="138.24452"
         style="stroke-width:0.0647447">Fill | Stroke</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.58978px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0647447"
       x="42.415726"
       y="138.24452"
       id="text6842-5-0-1-3-7"><tspan
         sodipodi:role="line"
         id="tspan6840-7-0-8-9-9"
         x="42.415726"
         y="138.24452"
         style="stroke-width:0.0647447">Change Colors</tspan><tspan
         sodipodi:role="line"
         x="42.415726"
         y="141.48175"
         style="stroke-width:0.0647447"
         id="tspan3900">With Values</tspan></text>
    <rect
       style="opacity:0.98;fill:#ffffff;fill-opacity:1;stroke:#000000;stroke-width:1.56606;stroke-miterlimit:4;stroke-dasharray:none"
       id="rect3847-6-5"
       width="10.962432"
       height="10.962432"
       x="70.189095"
       y="145.01048"
       inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;list&quot;:[{&quot;data&quot;:&quot;-999999&quot;,&quot;param&quot;:&quot;green|black&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;0&quot;,&quot;param&quot;:&quot;green|black&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;200&quot;,&quot;param&quot;:&quot;@red|black&quot;,&quot;tag&quot;:&quot;@1&quot;}]}" />
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.58978px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0647447"
       x="69.35199"
       y="138.24452"
       id="text6842-5-0-1-3-7-0"><tspan
         sodipodi:role="line"
         x="69.35199"
         y="138.24452"
         style="stroke-width:0.0647447"
         id="tspan3900-6">Color</tspan><tspan
         sodipodi:role="line"
         x="69.35199"
         y="141.48175"
         style="stroke-width:0.0647447"
         id="tspan3940">Interpolation</tspan></text>
    <rect
       style="fill:#ffffff;fill-opacity:1;stroke:#000000;stroke-width:0.626423;stroke-miterlimit:4;stroke-dasharray:none"
       id="rect3847-6-5-1"
       width="10.962432"
       height="10.962432"
       x="99.944267"
       y="144.69727"
       inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;list&quot;:[{&quot;data&quot;:&quot;-999999&quot;,&quot;param&quot;:&quot;attrib: opacity=0.1&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;0&quot;,&quot;param&quot;:&quot;attrib: opacity=0.3&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;199.9999&quot;,&quot;param&quot;:&quot;attrib: opacity=0.9&quot;,&quot;tag&quot;:&quot;@1&quot;}]}" />
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.58978px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0647447"
       x="99.107155"
       y="137.93132"
       id="text6842-5-0-1-3-7-0-6"><tspan
         sodipodi:role="line"
         x="99.107155"
         y="137.93132"
         style="stroke-width:0.0647447"
         id="tspan3968">Change</tspan><tspan
         sodipodi:role="line"
         x="99.107155"
         y="141.16855"
         style="stroke-width:0.0647447"
         id="tspan3996">Attributes</tspan></text>
    <rect
       style="fill:#ffffff;fill-opacity:1;stroke:#000000;stroke-width:0.626423;stroke-miterlimit:4;stroke-dasharray:none"
       id="rect3847-6-5-1-4"
       width="10.962432"
       height="10.962432"
       x="128.7598"
       y="144.69727"
       inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;list&quot;:[{&quot;data&quot;:&quot;-999999&quot;,&quot;param&quot;:&quot;green|black&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;0&quot;,&quot;param&quot;:&quot;green|black&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;199.9999&quot;,&quot;param&quot;:&quot;@red|black&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;200&quot;,&quot;param&quot;:&quot;script: $W.RemoveAnimate( thisobj ); $W.Animate( thisobj, 'animate', {'attributeName': 'width', 'from': 74, 'to': 100, 'repeatCount':5,'dur': 1 } );&quot;,&quot;tag&quot;:&quot;@1&quot;}]}" />
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.58978px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0647447"
       x="127.9227"
       y="137.93132"
       id="text6842-5-0-1-3-7-0-6-3"><tspan
         sodipodi:role="line"
         x="127.9227"
         y="137.93132"
         style="stroke-width:0.0647447"
         id="tspan3940-0-2">Trigger</tspan><tspan
         sodipodi:role="line"
         x="127.9227"
         y="141.16855"
         style="stroke-width:0.0647447"
         id="tspan3968-2">Animation</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-variant:normal;font-weight:normal;font-stretch:normal;font-size:4.26846px;line-height:1.25;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Normal';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:center;letter-spacing:0px;word-spacing:0px;writing-mode:lr-tb;text-anchor:middle;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.106711"
       x="76.374832"
       y="161.14989"
       id="text8053-50"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-4"
         x="76.374832"
         y="161.14989"
         style="font-style:normal;font-variant:normal;font-weight:normal;font-stretch:normal;font-size:4.26846px;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Normal';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:center;writing-mode:lr-tb;text-anchor:middle;stroke-width:0.106711">%.1f</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-variant:normal;font-weight:normal;font-stretch:normal;font-size:4.01455px;line-height:1.25;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Normal';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:center;letter-spacing:0px;word-spacing:0px;writing-mode:lr-tb;text-anchor:middle;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.100363"
       x="105.20885"
       y="160.7991"
       id="text8053-8"
       inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;list&quot;:[{&quot;data&quot;:&quot;-999999&quot;,&quot;param&quot;:&quot;attrib: style=fill:green;text-decoration:none;&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;100&quot;,&quot;param&quot;:&quot;attrib: style=fill:red;text-decoration:underline;&quot;,&quot;tag&quot;:&quot;@1&quot;}]},{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;Allen Junction_H2S&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-1"
         x="105.20885"
         y="160.7991"
         style="font-style:normal;font-variant:normal;font-weight:normal;font-stretch:normal;font-size:4.01455px;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Normal';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:center;writing-mode:lr-tb;text-anchor:middle;stroke-width:0.100363">%.1f</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="13.312727"
       y="72.622002"
       id="text8053-5-6-75-0"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;!TAG @1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8-1-2-9"
         x="13.312727"
         y="72.622002"
         style="fill:#0000ff;stroke-width:0.140579">%s</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-variant:normal;font-weight:bold;font-stretch:normal;font-size:5.18951px;line-height:1.25;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Bold';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:start;letter-spacing:0px;word-spacing:0px;writing-mode:lr-tb;text-anchor:start;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.129736"
       x="9.5817547"
       y="91.995193"
       id="text6842-5-0-8"><tspan
         sodipodi:role="line"
         id="tspan6840-7-0-81"
         x="9.5817547"
         y="91.995193"
         style="font-style:normal;font-variant:normal;font-weight:bold;font-stretch:normal;font-size:5.18951px;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Bold';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:start;writing-mode:lr-tb;text-anchor:start;stroke-width:0.129736">Text Tab</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:4.40305px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.110076"
       x="13.577642"
       y="104.8528"
       id="text8053-5-6-7-7"
       inkscape:label="{&quot;attr&quot;:&quot;text&quot;,&quot;map&quot;:[&quot;-999999999=Less than zero&quot;,&quot;0=Greater than zero, less than 100&quot;,&quot;100=More than a hundred&quot;],&quot;tag&quot;:&quot;@1&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8-1-4-2"
         x="13.577642"
         y="104.8528"
         style="fill:#0000ff;stroke-width:0.110076">text</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="90.162704"
       y="61.798027"
       id="text8053-5-6-7-8-3"
       inkscape:label="{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8-1-4-0-1"
         x="90.162704"
         y="61.798027"
         style="fill:#0000ff;stroke-width:0.140579">,.3s</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-variant:normal;font-weight:normal;font-stretch:normal;font-size:2.77444px;line-height:1.25;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Normal';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:center;letter-spacing:0px;word-spacing:0px;writing-mode:lr-tb;text-anchor:middle;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0693603"
       x="86.537048"
       y="15.018414"
       id="text7530"><tspan
         sodipodi:role="line"
         x="86.537048"
         y="15.018414"
         id="tspan7532"
         style="font-style:normal;font-variant:normal;font-weight:normal;font-stretch:normal;font-size:2.77444px;font-family:sans-serif;-inkscape-font-specification:'sans-serif, Normal';font-variant-ligatures:normal;font-variant-caps:normal;font-variant-numeric:normal;font-feature-settings:normal;text-align:center;writing-mode:lr-tb;text-anchor:middle;stroke-width:0.0693603">To edit SCADA animations for an object, right-click the object and select &quot;Object Properties&quot;</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.7744px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0693603"
       x="9.4445095"
       y="29.34486"
       id="text7530-3"><tspan
         sodipodi:role="line"
         x="9.4445095"
         y="29.34486"
         id="tspan7532-4"
         style="stroke-width:0.0693603">Use the get tab to retrive and show formatted values of tags (available only for text objects).</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.52271px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0630672"
       x="50.77467"
       y="17.647392"
       id="text7558"><tspan
         sodipodi:role="line"
         id="tspan7556"
         x="50.77467"
         y="17.647392"
         style="stroke-width:0.0630672">https://scadavis.io/scadaviseditor.docx.html</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.7744px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0693603"
       x="9.4445095"
       y="96.387558"
       id="text7530-3-4"><tspan
         sodipodi:role="line"
         x="9.4445095"
         y="96.387558"
         id="tspan7532-4-6"
         style="stroke-width:0.0693603">Use the text tab to display predefined texts associated with ranges of values (available only for text objects).</tspan></text>
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:2.7744px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#000000;fill-opacity:1;stroke:none;stroke-width:0.0693603"
       x="9.5298481"
       y="127.79514"
       id="text7530-3-4-6"><tspan
         sodipodi:role="line"
         x="9.5298481"
         y="127.79514"
         id="tspan7532-4-6-5"
         style="stroke-width:0.0693603">Change color and other attributes of objects based on value ranges. Can trigger scripts also.</tspan></text>
    <path
       style="fill:#dde9af;fill-opacity:0.980392;stroke:#44aa00;stroke-width:0.156404px;stroke-linecap:butt;stroke-linejoin:miter;stroke-opacity:1"
       d="M 7.0277405,85.096626 H 164.0533 l -0.31322,-0.443052"
       id="path7604"
       inkscape:connector-curvature="0"
       sodipodi:nodetypes="ccc" />
    <path
       style="fill:#dde9af;fill-opacity:0.980392;stroke:#44aa00;stroke-width:0.156404px;stroke-linecap:butt;stroke-linejoin:miter;stroke-opacity:1"
       d="M 7.1273985,116.6605 H 164.15294 v -0.44305"
       id="path7604-9"
       inkscape:connector-curvature="0" />
    <text
       xml:space="preserve"
       style="font-style:normal;font-weight:normal;font-size:5.62318px;line-height:1.25;font-family:sans-serif;letter-spacing:0px;word-spacing:0px;fill:#0000ff;fill-opacity:1;stroke:none;stroke-width:0.140579"
       x="38.849468"
       y="163.4511"
       id="text8053-5-6-4"
       inkscape:label="{&quot;attr&quot;:&quot;color&quot;,&quot;list&quot;:[{&quot;data&quot;:&quot;-99999&quot;,&quot;param&quot;:&quot;green|none&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;0&quot;,&quot;param&quot;:&quot;#00ff00|&quot;,&quot;tag&quot;:&quot;@1&quot;},{&quot;data&quot;:&quot;200&quot;,&quot;param&quot;:&quot;none|red&quot;,&quot;tag&quot;:&quot;@1&quot;}]},{&quot;align&quot;:&quot;Right&quot;,&quot;attr&quot;:&quot;get&quot;,&quot;tag&quot;:&quot;@1&quot;,&quot;type&quot;:&quot;Good&quot;}"><tspan
         sodipodi:role="line"
         id="tspan8051-8-1-5"
         x="38.849468"
         y="163.4511"
         style="stroke-width:0.140579">%08.2f</tspan></text>
  </g>
</svg>
```
