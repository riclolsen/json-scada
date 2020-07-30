// A tool to convert OSHMI's point_list.txt files to JSON-SCADA MongoDB import files.
// {json:scada} - Copyright 2020 - Ricardo L. Olsen

let sscanf = require('scanf').sscanf
var printf = require('printf')
const fs = require('fs')
const readline = require('readline')

let protocolSourceConnectionNumber = 1;
let fnameInput = 'point_list.txt'
let fnameJSOutput = 'json-scada-mongo-import.js'
let fnameCSVOutput = 'json-scada-mongo-import.csv'

let writeStream = fs.createWriteStream(fnameJSOutput, { flags: 'w' })
let writeStreamCSV = fs.createWriteStream(fnameCSVOutput, { flags: 'w' })

let tags = []

printf(writeStream, "// use json_scada\n");
printf(writeStream, 'var bulk=db.realtimeData.initializeUnorderedBulkOp();\n')

let lineReader = require('readline').createInterface({
  input: require('fs').createReadStream(fnameInput)
})

let cntLine = 0
lineReader
  .on('line', function (line) {

    // ignore headers (2 lines at the beginning)
    if (cntLine >= 2) {
      let res = sscanf(
        line,
        '%d %d %s %s %s %d %d %d %d %s %d %d %f %f %d %d %d %f %S',
        'pointKey',
        'address',
        'tag',
        'type',
        'messageOrUnit',
        'almCode',
        'equipCode',
        'infoCode',
        'originCode',
        'specialCode',
        'rtuAddress',
        'asduTiIEC',
        'kconv1',
        'kconv2',
        'supervisedOfCommand',
        'alarmState',
        'priority',
        'defaultValue',
        'g1g2description'
      )
      res.commandOfSupervised = 0;

      if (res.address === 0)
        res.address = res.pointKey

      // process on/off/unit
      if (res.type === 'D') {
        let onoff = res.messageOrUnit.split('/')
        res.offMessage = onoff[0]
        if (onoff.length > 1) res.onMessage = onoff[1]
        else res.onMessage = ''
      } else res.unit = res.messageOrUnit

      // unquote string
      res.g1g2description = res.g1g2description.trim().replace(/^"(.+(?="$))"$/, '$1')
      let g1g2da = res.g1g2description.split('~')
      res.group1 = g1g2da[0]
      if (g1g2da.length > 0) res.group2 = g1g2da[1]
      else res.group2 = ''
      if (g1g2da.length > 1) res.ungroupedDescription = g1g2da[2]
      else res.ungroupedDescription = ''

     switch (res.originCode){

        case 1: 
           res.origin = "calculated"
           break;
        case 6: 
           res.origin = "manual"
           break;
        case 7: 
           res.origin = "command"
           break;
           case 11: 
           res.origin = "estimated"
           break;

         case 0:           
         default:
             res.origin = "supervised"
             break;
     }

     res.frozenDetectTimeout = 300;  
     if ( res.type=="D" || res.equipCode === 16 || res.originCode === 6 )
       res.frozenDetectTimeout = 0;      

     tags.push(res) 
    }
    cntLine++
})
  .on('close', function () {
 

    printf( writeStreamCSV,
          "_id,tag,type,origin,description,ungroupedDescription,group1,group1,group3," +
          "valueDefault,priority,frozenDetectTimeout,invalidDetectTimeout,historianDeadBand,historianPeriod,"+
          "supervisedOfCommand,commandOfSupervised,location,isEvent,unit,"+
          "alarmState,stateTextTrue,stateTextFalse,eventTextTrue,eventTextFalse,formula,parcels,kconv1,kconv2,"+
          "protocolSourceConnectionNumber,protocolSourceCommonAddress,protocolSourceObjectAddress,protocolSourceASDU,"+
          "protocolSourceCommandDuration,protocolSourceCommandUseSBO,"+
          "hiLimit,hihiLimit,hihihiLimit,loLimit,loloLimit,lololoLimit,hysteresis,"+
          "substituted,alarmDisabled,annotation,commandBlocked,notes,updatesCnt,alarmed,invalid,overflow,transient,frozen,"+
          "sourceDataUpdate,value,valueString,timeTag,timeTagAlarm,timeTagAtSource,timeTagAtSourceOk\n"
          );
        tags.map(element => {
            printf( writeStreamCSV,
                '%d,'+ // _id 0
                '"%s",'+ // tag 1
                '"%s",'+ // type 2
                '"%s",'+ // origin 3 
                '"%s",'+ // description 4
                '"%s",'+ // ungroupedDescription 5
                '"%s",'+ // group1 6
                '"%s",'+ // group2 7
                '"%s",'+ // group3 8
                '%f,'+ // valueDefault 9
                '%d,'+ // priority 10
                '%d,'+ // frozenDetectTimeout 11
                '%d,'+ // invalidDetectTimeout 12
                '%f,'+ // historianDeadBand 13
                '%d,'+ // historianPeriod 14
                '%d,'+ // supervisedOfCommand 15
                '%d,'+ // commandOfSupervised 16
                '%s,'+ // location 17
                '%s,'+ // isEvent 18
                '"%s",'+ // unit 19
                '%d,'+ // alarmState 20
                '"%s",'+ // stateTextTrue 21
                '"%s",'+ // stateTextFalse 22
                '"%s",'+ // eventTextTrue 23
                '"%s",'+ // eventTextFalse 24
                '%s,'+ // formula 25
                '%s,'+ // parcels 26
                '%f,'+ // kconv1 27
                '%f,'+ // kconv2 28
                '%d,'+ // protocolSourceConnectionNumber 29
                '%d,'+ // protocolSourceCommonAddress 30
                '%d,'+ // protocolSourceObjectAddress 31
                '%d,'+ // protocolSourceASDU 32
                '%s,'+ // protocolSourceCommandDuration 33
                '%s,'+ // protocolSourceCommandUseSBO 34
                '%s,'+ // hiLimit 35
                '%s,'+ // hihiLimit 36
                '%s,'+ // hihihiLimit 37
                '%s,'+ // loLimit 38
                '%s,'+ // loloLimit 39
                '%s,'+ // lololoLimit 40
                '%f,'+ // hysteresis 41
                '%s,'+ // substituted 42
                '%s,'+ // alarmDisabled 43
                '"%s",'+ // annotation 44
                '%s,'+ // commandBlocked 45
                '"%s",'+ // notes 46
                '%d,'+ // updatesCnt 47
                '%s,'+ // alarmed 48
                '%s,'+ // invalid 49
                '%s,'+ // overflow 50
                '%s,'+ // transient 51
                '%s,'+ // frozen 52
                '%s,'+  // sourceDataUpdate 53
                '%f,'+ // value 54
                '"%f",'+ // valueString 55
                '%s,'+ // timeTag 56
                '%s,'+ // timeTagAlarm 57
                '%s,'+ // timeTagAtSource 58
                '%s'+ // timeTagAtSourceOk 59
                '\n'
                ,    
                element.pointKey,  // pointKey 0
                element.tag,  // tag 1
                element.type=="D"?"digital":"analog", // type 2
                element.origin, // origin 3
                element.group1+"~"+element.group2+"~"+element.ungroupedDescription, // description 4
                element.ungroupedDescription, // ungroupedDescription 5
                element.group1, // group1 6
                element.group2, // group2 7
                '', // group3 8
                element.defaultValue, // valueDefault 9
                element.priority, // priority 10
                element.frozenDetectTimeout, // frozenDetectTimeout 11
                element.originCode===6?0:300, // invalidDetectTimeout 12
                0, // historianDeadBand 13
                0, // historianPeriod 14
                element.supervisedOfCommand, // supervisedOfCommand 15
                element.commandOfSupervised, // commandOfSupervised 16
                'null', // location 17
                element.alarmState===3?'true':'false', // isEvent 18
                element.type==="D"?"":element.unit, // unit 19
                element.alarmState, // alarmedState 20
                element.onMessage, // stateTextTrue 21
                element.offMessage, // stateTextFalse 22
                element.onMessage, // eventTextTrue 23
                element.offMessage, // eventTextFalse 24
                0, // formula 25
                'null', // parcels 26
                element.kconv1, // 27
                element.kconv2, // 28
                protocolSourceConnectionNumber, // protocolSourceConnectionNumber 29
                element.rtuAddress, // protocolSourceCommonAddress 30
                element.address, // protocolSourceObjectAddress 31
                element.asduTiIEC, // protocolSourceASDU 32
                element.originCode===7?element.kconv2:"null", // protocolSourceCommandDuration 33
                element.originCode===7?(element.kconv1===1?'true':'false'):'null', // protocolSourceCommandUseSBO 34
                '0', // hiLimit 35
                '0', // hihiLimit 36
                '0', // hihihiLimit 37
                '0', // loLimit 38
                '0', // loloLimit 39
                '0', // lololoLimit 40
                0, // hysteresis 41
                'false', // substituted 42
                'false', // alarmDisabled 43
                '', // annotation 44
                'false', // commandBlocked 45
                '', // notes 46
                0, // updatesCnt 47
                'false', // alarmed 48
                element.originCode===6?'false':'true', // invalid 49
                'false', // overflow 50
                'false', // transient 51
                'false', // frozen 52
                'null', // sourceDataUpdate 53
                element.defaultValue, // value 54
                element.defaultValue, // valueString 55
                'null', // timeTag 56
                'null', // timeTagAlarm 57
                'null', // timeTagAtSource 58
                'null'  // timeTagAtSourceOk 59
                )
    })
    writeStreamCSV.end()
        
    tags.map(element => {

        printf( writeStream,
            'bulk.find({_id:%u}).upsert().updateOne({$set:'+
              '{tag:"%s"'+
              ',type:"%s"'+
              ',origin:"%s"'+
              ',description:"%s"'+
              ',ungroupedDescription:"%s"'+
              ',group1:"%s"'+
              ',group2:"%s"'+
              ',group3:"%s"'+
              ',valueDefault:%.14g'+
              ',priority:%d'+
              ',frozenDetectTimeout:%d'+
              ',invalidDetectTimeout:%d'+
              ',historianDeadBand:%.14g'+ 
              ',historianPeriod:%d'+
              ',supervisedOfCommand:%d'+
              ',commandOfSupervised:%d'+
              ',location:%s'+
              ',isEvent:%s'+
              ',unit:"%s"'+
              ',alarmState:%d'+          
              ',stateTextTrue:"%s"'+
              ',stateTextFalse:"%s"'+
              ',eventTextTrue:"%s"'+
              ',eventTextFalse:"%s"'+
              ',formula:%s'+
              ',parcels:%s'+
              ',kconv1:%.14g'+
              ',kconv2:%.14g'+
              ',protocolSourceConnectionNumber:%d'+
              ',protocolSourceCommonAddress:%d'+
              ',protocolSourceObjectAddress:%d'+
              ',protocolSourceASDU:%d'+
              ',protocolSourceCommandDuration:%s'+
              ',protocolSourceCommandUseSBO:%s'+
            '}'+
            // Campos preservados: insere somente quando o+documento é criado (não faz update)
            ',$setOnInsert: {'+
              ' hiLimit:%s'+ 
              ',hihiLimit:%s'+
              ',hihihiLimit:%s'+
              ',loLimit:%s'+
              ',loloLimit:%s'+
              ',lololoLimit:%s'+
              ',hysteresis:%.14g'+
              ',substituted:%s'+
              ',alarmDisabled:%s'+
              ',annotation:"%s"'+
              ',commandBlocked:%s'+
              ',notes:"%s"'+
              ',updatesCnt:%d'+
              ',alarmed:%s'+
              ',invalid:%s'+
              ',overflow:%s'+
              ',transient:%s'+
              ',frozen:%s'+
              ',sourceDataUpdate:%s'+ 
              ',value:%.14g'+
              ',valueString:"%f"'+
              ',timeTag:%s'+
              ',timeTagAlarm:%s'+
              ',timeTagAtSource:%s'+
              ',timeTagAtSourceOk:%s'+
            '}'+
            '});'+'\n', 
            element.pointKey, 
            element.tag, 
            element.type=="D"?"digital":"analog",
            element.origin,
            element.group1+"~"+element.group2+"~"+element.ungroupedDescription, // description
            element.ungroupedDescription, // ungroupedDescription
            element.group1, // group1
            element.group2, // group2
            '', // group3
            element.defaultValue, // valueDefault
            element.priority,
            element.frozenDetectTimeout,
            element.originCode===6?0:300, // invalidDetectTimeout
            0, // historianDeadBand
            0, // historianPeriod
            element.supervisedOfCommand, // supervisedOfCommand
            element.commandOfSupervised, // commandOfSupervised
            'null', // location
            element.alarmState===3?'true':'false', // isEvent
            element.type==="D"?"":element.unit, // unit
            element.alarmState, // alarmedState
            element.onMessage, // stateTextTrue
            element.offMessage, // stateTextFalse
            element.onMessage, // eventTextTrue
            element.offMessage, // eventTextFalse
            0, // formula
            'null', // parcels
            element.kconv1,
            element.kconv2,
            protocolSourceConnectionNumber, // protocolSourceConnectionNumber
            element.rtuAddress, // protocolSourceCommonAddress
            element.address, // protocolSourceObjectAddress
            element.asduTiIEC, // protocolSourceASDU
            element.originCode===7?element.kconv2:"null", // protocolSourceCommandDuration
            element.originCode===7?(element.kconv1===1?'true':'false'):'null', // protocolSourceCommandUseSBO
            'Infinity', // hiLimit
            'Infinity', // hihiLimit
            'Infinity', // hihihiLimit
            '-Infinity', // loLimit
            '-Infinity', // loloLimit
            '-Infinity', // lololoLimit
            0, // hysteresis
            'false', // substituted
            'false', // alarmDisabled
            '', // annotation
            'false', // commandBlocked
            '', // notes
            0, // updatesCnt
            'false', // alarmed
            element.originCode===6?'false':'true', // invalid
            'false', // overflow
            'false', // transient
            'false', // frozen
            'null', // sourceDataUpdate
            element.defaultValue, // value
            element.defaultValue, // valueString
            'null', // timeTag
            'null', // timeTagAlarm,
            'null', // timeTagAtSource 
            'null'  // timeTagAtSourceOk
          );
    })

   printf(writeStream, 'bulk.execute();\n')
   console.log('end')
   writeStream.end()
  })