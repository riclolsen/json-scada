// Viewers Configuration Parameters

// list of colors representing priorities for the alarms/tabular/events viewers
var ColorOfPriority = [ "red", "yellow", "goldenrod", "plum", "silver", "silver", "silver", "silver", "silver", "silver", "silver" ];
// list of colors representing the first up to last group1/substation for the alarms/tabular/events viewers
var ColorOfSubstation = [ "cadetblue", "brown", "green", "magenta", "orange", "darkcyan", "goldenrod", "deepskyblue", "indigo", "lightseagreen" ];

// Events Viewer --------------------------------------------------------------------------------
var EventsViewer_ToolbarColor = '#BBBBBB'; // toolbar color

var EventsViewer_Font = 'Segoe UI,Source Sans Pro,calibri,consolas,arial,helvetica'; // font 

var EventsViewer_TableColor = '#E8E8E8'; // table background color
var EventsViewer_GridColor = '#00000011'; // table grid color

var EventsViewer_AlmTxtColor = 'mediumblue'; // alarmed color
var EventsViewer_FailTxtColor = 'white'; // failed value color
var EventsViewer_AckTxtColor = '#606060'; // acknowledged event color
var EventsViewer_ElimTxtColor = '#B0B0B0'; // eliminated event color (until removed)

var EventsViewer_RefreshTime = 12; // refresh time in seconds (digital changes triggers a faster refresh)

var EventsViewer_MaxRealtimeEvents = 750; // maximum number of realtime events to show 
var EventsViewer_MaxHistoricalEvents = 2500; // maximum number of historical events to query from server at each request

// Event time tag configuration
// 0 = GPS (field time)
// 1 = local time (time when HMI detected the event)
// 2 = chosen by the operator (defaults to GPS time)
// 3 = chosen by the operator (defaults to local)
var EventsViewer_TimeGPSorLocal = 2;

var EventsViewer_AllowFilter = 1;  // 0: no filters, 1=operator can set filter by substation

var EventsViewer_Notific = 1;  // 0: disable desktop notifications, 1=enable desktop notifications

var EventsViewer_PanicModePriorityLimit = 1; // limit priorities shown in panic mode (up to)

// locale and time zone for date/time formatting (leave empty for browser defaults)
// see https://developer.mozilla.org/pt-BR/docs/Web/JavaScript/Reference/Global_Objects/Date/toLocaleString
var EventsViewer_LocaleTime = ""; // locale for time format e.g "en-US"
var EventsViewer_LocaleDate = ""; // locale for date format e.g "en-US"
// for options to work, a locale must be set
var EventsViewer_LocaleTimeOptions = {
    // timeZone: 'America/New_York'
}; 
var EventsViewer_LocaleDateOptions = {
    // weekday: 'long', year: 'numeric', month: 'long', day: 'numeric', timeZone: 'America/New_York'
}; 

// defines how to handle notification click 
// defaults to open a screen with the group1 (substation) name, adjust for your display naming standard
function EventsViewer_NotificationClick( nponto, id, group1 ) {
    window.open('display.html?SELTELA=../svg/'+group1+'.svg','screen','dependent=no,height=1000,width=800,toolbar=no,directories=no,status=no,menubar=no,scrollbars=no,resizable=no,modal=no');
    }

// Tabular Viewer ----------------------------------------------------------------------------------
var TabularViewer_Font = 'Segoe UI,Source Sans Pro,calibri,consolas,arial,helvetica'; // font 

var TabularViewer_GridColor = '#BBBBBB'; // table grid color

var TabularViewer_AlmTxtColor = 'rgb(164, 128, 206)'; // alarmed color
var TabularViewer_FailTxtColor = '#AAAAAA'; // failed value color
var TabularViewer_AckTxtColor = 'rgb(40, 130, 136)'; // acknowledged alarm color

var TabularViewer_RefreshTime = 3; // refresh time (seconds)

// locale and time zone for date/time formatting (leave empty for browser defaults)
// see https://developer.mozilla.org/pt-BR/docs/Web/JavaScript/Reference/Global_Objects/Date/toLocaleString
var TabularViewer_LocaleDateTime = ""; // locale for time format e.g "en-US"
// for options to work, a locale must be set
var TabularViewer_LocaleDateTimeOptions = {
    // weekday: 'long', year: 'numeric', month: 'long', day: 'numeric', hour: 'numeric', minute: 'numeric', second: 'numeric', fractionalSecondDigits: 3, timeZone: 'America/New_York'
}; 

// use this to create custom buttons that can select a set of group1 filters on Alarms Viewer
var TabularViewer_CustomFiltersSelectors = [
//    { name: 'Test1', group1List: ['KAW2', 'KOR1'] },
//    { name: 'Test2', group1List: ['KIK3', 'KNH2'] },
];

// Screen Viewer ---------------------------------------------------------------------------------

var ScreenViewer_RefreshTime = 3; // refresh time in seconds (digital changes triggers a faster refresh) 

// SVG Screen dimensions (must match <svg> tag dimensions on SVG screens)
// <svg width="3840" height="2160">
var ScreenViewer_SVGMaxWidth = 3840; // default SVG screen width in pixels
var ScreenViewer_SVGMaxHeight  = 2160; // default SVG screen height in pixels

var ScreenViewer_Background  = '#DDDDDD'; // background color for Inkscape SAGE SVG screens
var ScreenViewer_ToolbarColor = 'lightslategray'; // toolbar color
var ScreenViewer_RelationColor = 'red'; // color for relationated points when shown by [shift+backspace]
var ScreenViewer_TagFillColor = 'yellow'; // annotation tag fill color
var ScreenViewer_TagStrokeColor = 'red'; // annotation tag stroke color
var ScreenViewer_TagInhAlmFillColor = 'lightgray'; // inhibited alarm tag fill color
var ScreenViewer_TagInhAlmStrokeColor = 'gray'; // inhibited alarm tag border color
var ScreenViewer_DateColor = "white"; // data update time color
var ScreenViewer_TimeMachineDateColor = "orange"; // time machine data update color
var ScreenViewer_TimeMachineBgColor = "green"; // time machine background color
var ScreenViewer_AlmBoxTableColor = "#DCDCEE"; // alarm box table color
var ScreenViewer_AlmBoxGridColor = "whitesmoke"; // alarm box grid color
var ScreenViewer_BarBreakerSwColor = "steelblue"; // color for DJ, SC and bars
var ScreenViewer_ShowScreenNameTB = 1; // show screen name on toolbar

// parameters for display preview 
var ScreenViewer_DefaultDisplayPreviewWidth = 700;
var ScreenViewer_DefaultDisplayPreviewHeight = 480;
var ScreenViewer_DisplayPreviewZoom = 0.5;

var ScreenViewer_PinnedAnnotationsBGColor = "slategray"; // background color of pinned annotations
var ScreenViewer_PinnedAnnotationsTextColor = "white"; // text color of pinned annotations
var ScreenViewer_PinnedAnnotationsWidth = "300px"; // width of pinned annotations
var ScreenViewer_PinnedAnnotationsBorder = "1px solid black"; // border for pinned annotations
var ScreenViewer_PinnedAnnotationsFont = "calibri"; // font for pinned annotations
var ScreenViewer_PinnedAnnotationsFontSize = "12px"; // font size for pinned annotations

// user color palette (use in Inkscape+SAGE color fields as "-cor-05" or "-cor-49")
var ScreenViewer_ColorTable = new Array();
ScreenViewer_ColorTable[0]  = 'white'; // 
ScreenViewer_ColorTable[1]  = 'white'; // sc ou dj falha
ScreenViewer_ColorTable[2]  = 'white'; // 
ScreenViewer_ColorTable[3]  = ScreenViewer_Background; // dj ab
ScreenViewer_ColorTable[4]  = ScreenViewer_BarBreakerSwColor; // sc ou dj fc
ScreenViewer_ColorTable[5]  = ScreenViewer_BarBreakerSwColor; // sc ab
ScreenViewer_ColorTable[6]  = 'cornsilk'; // sc ou dj 00 
ScreenViewer_ColorTable[7]  = 'cornsilk'; // sc ou dj 11
ScreenViewer_ColorTable[8]  = ScreenViewer_BarBreakerSwColor; // borda do dj ab
ScreenViewer_ColorTable[9]  = '#AAAAAA'; // unidades de medidas
ScreenViewer_ColorTable[10] = '#6199c7'; // outras medida ok
ScreenViewer_ColorTable[11] = 'red'; // medida fora de faixa
ScreenViewer_ColorTable[12] = 'white'; // medida ou estado falhado
ScreenViewer_ColorTable[13] = ScreenViewer_BarBreakerSwColor; // titulo da tela (SE)
ScreenViewer_ColorTable[14] = ScreenViewer_BarBreakerSwColor; // texto de linha
ScreenViewer_ColorTable[15] = '#777777'; // número de equipamento
ScreenViewer_ColorTable[16] = ScreenViewer_BarBreakerSwColor; // barra 230kV
ScreenViewer_ColorTable[17] = ScreenViewer_BarBreakerSwColor; // barra 138kV 
ScreenViewer_ColorTable[18] = ScreenViewer_BarBreakerSwColor; // barra de 69kV
ScreenViewer_ColorTable[19] = ScreenViewer_BarBreakerSwColor; // barra de alimentadores 13kV/23kV
ScreenViewer_ColorTable[20] = '#6199c7'; // medida de MW ok
ScreenViewer_ColorTable[21] = '#6199c7'; // medida de MVAr ok
ScreenViewer_ColorTable[22] = '#6199c7'; // medida de kV ok
ScreenViewer_ColorTable[23] = '#6199c7'; // medida de corrente ok
ScreenViewer_ColorTable[24] = '#6199c7'; // medida de tap ok
ScreenViewer_ColorTable[25] = '#6199c7'; // cor da barra de 500kV
ScreenViewer_ColorTable[26] = 'gray'; // texto estados
ScreenViewer_ColorTable[27] = 'gray'; // grade de estados
ScreenViewer_ColorTable[28] = 'darkgreen'; // estados off
ScreenViewer_ColorTable[29] = 'darkred'; // estado on
ScreenViewer_ColorTable[30] = 'red'; // estado de alarme
ScreenViewer_ColorTable[31] = 'lightgray'; // quadro de estados
ScreenViewer_ColorTable[32] = 'black'; // borda do quadro de estados
ScreenViewer_ColorTable[33] = '#D7D7D7'; // área de operação
ScreenViewer_ColorTable[34] = 'gray'; // símbolo de aterramento
ScreenViewer_ColorTable[35] = ScreenViewer_BarBreakerSwColor; // estado normal
ScreenViewer_ColorTable[36] = 'mediumvioletred'; // estado anormal
ScreenViewer_ColorTable[37] = 'red'; // estado alarmado
ScreenViewer_ColorTable[38] = '#999999'; // texto estático
ScreenViewer_ColorTable[39] = '#DDE8DD'; // quadro de status normal
ScreenViewer_ColorTable[40] = 'yellow'; // quadro de status alarme
ScreenViewer_ColorTable[41] = 'deepskyblue'; // medida congelada
ScreenViewer_ColorTable[42] = 'red'; // medida alarmada
ScreenViewer_ColorTable[43] = '#6199c7'; // outras medidas ok
ScreenViewer_ColorTable[44] = 'red'; // alarm priority 0
ScreenViewer_ColorTable[45] = 'yellow'; // alarm priority 1
ScreenViewer_ColorTable[46] = 'orange'; // alarm priority 2
ScreenViewer_ColorTable[47] = 'fuchsia'; // alarm priority 3 (diagnóstico)
ScreenViewer_ColorTable[48] = '#CCCCCC'; // state box fill color inactive 
ScreenViewer_ColorTable[49] = '#CCCCCC'; // state box border color inactive 
ScreenViewer_ColorTable[50] = '#505050'; // state box active text color   
ScreenViewer_ColorTable[51] = 'lightsteelblue'; // state box fill color ON 
ScreenViewer_ColorTable[52] = 'tan'; // state box fill color OFF
ScreenViewer_ColorTable[53] = '#888888'; // state box inactive text color  
ScreenViewer_ColorTable[54] = 'red'; // state box operated text color  
ScreenViewer_ColorTable[55] = 'lightsteelblue'; // analog range ok
ScreenViewer_ColorTable[56] = '#777777'; // analog range out of limits
ScreenViewer_ColorTable[57] = ScreenViewer_BarBreakerSwColor; // analog indicator ok
ScreenViewer_ColorTable[58] = 'crimson'; // analog indicator out of limits
ScreenViewer_ColorTable[59] = '#CCCCCC'; // load rectangle MW or MVA or A

var ScreenViewer_SlideShowInterval = 10; // slide show time in seconds
var ScreenViewer_EnableTimeMachine = 1; // 1 = enable time machine, 0 = disable time machine
var ScreenViewer_SecurityCardScale = 1.0; // scale to represent an annotation security card

// Command dialog
var CommandDialog_Background = 'wheat'; // background color

// Other options ------------------------------------------------------------------------

// Type 1 highlighted alarms
var Viewers_AlmTxtHighlight1 = 'OPERATED'; // Alarm text to identify Type 1 highlighted alarms

// Type 2 highlighted alarms
var Viewers_IDTxtHighlight2    = 'XCBR'; // Text present on ID to identify Type 2 highlighted alarms
// and
var Viewers_DescrTxtHighlight2 = ':status'; // Text present on DESCRIPTION to identify Type 2 highlighted alarms

// For Desktop Notifications of SOE Events
var Viewers_NotificTagText = 'XCBR'; // Text present on tag of SOE Event for notification
// and
var Viewers_NotificDescrText = ":status"; // Text present on tag of SOE Event for notification
// and
var Viewers_NotificEventText = "OFF"; // Text present on tag of SOE Event for notification
// and
var Viewers_NotificCancelEventText = "ON"; // Text present on tag of SOE Event to cancel notification
// and
var Viewers_AddToNotificEventText = 'OPERATED'; // Text present on event of SOE Event for notification
