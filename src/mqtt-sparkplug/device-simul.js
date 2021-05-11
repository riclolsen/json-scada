/*
 * MQTT-Sparkplug B Device Simulator
 * 
 * {json:scada} - Copyright (c) 2020-2021 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * 
 * Derived from https://github.com/Cirrus-Link/Sparkplug/tree/master/client_libraries/javascript/sparkplug-client
 *
 */

/********************************************************************************
 * Copyright (c) 2016-2018 Cirrus Link Solutions and others
 *
 * This program and the accompanying materials are made available under the
 * terms of the Eclipse Public License 2.0 which is available at
 * http://www.eclipse.org/legal/epl-2.0.
 *
 * SPDX-License-Identifier: EPL-2.0
 *
 * Contributors:
 *   Cirrus Link Solutions - initial implementation
 ********************************************************************************/
 const Fs = require('fs');
 const SparkplugClient = require('./sparkplug-client');

 /*
  * Main sample function which includes the run() function for running the sample
  * https://medium.com/solacedotcom/mqtt-client-certificate-authentication-with-solaces-pubsub-broker-98d7244180b4
  */
 let sample = (function () {

     let config = {             
             //'serverUrl' :'mqtt://192.168.0.47:1883',
             'serverUrl' :'mqtt://127.0.0.1:1883',
             //'serverUrl' :'mqtt://broker.hivemq.com',
             'username' : '',
             'password' : '',
             'groupId' : 'Sparkplug Devices',
             'edgeNode' : 'JavaScript Edge Node',
             'clientId' : 'JavaScriptSimpleEdgeNode',
             'version' : 'spBv1.0',
             'publishDeath': true,
             //'scadaHostId' : scadaHostId,

             //'pfx': Fs.readFileSync('certificate.pfx'),
             //'passphrase': 'test',
             //'ca': Fs.readFileSync('root.key'),
             //'key': Fs.readFileSync('client.key'),
             //'cert':  Fs.readFileSync('client.pem'),
             //'secureProtocol': 'TLSv1_2_method',
             //'rejectUnauthorized': false,

         },
         hwVersion = 'Emulated Hardware',
         swVersion = 'v1.0.0',
         deviceId = 'Emulated Device',
         sparkPlugClient,
         publishPeriod = 5000,         
         
     // Generates a random integer
     randomInt = function() {
         return 1 + Math.floor(Math.random() * 10);
     },
 
     // Get BIRTH payload for the edge node
     getNodeBirthPayload = function() {
         return {
             "timestamp" : new Date().getTime(),
             "metrics" : [
                {
                     "name" : "Node Control/Rebirth",
                     "type" : "boolean",
                     "value" : false
                },
                {
                    "name" : "Node Control/Reboot",
                    "type" : "boolean",
                    "value" : false
                },
                {
                    "name" : "Node Control/Next Server",
                    "type" : "boolean",
                    "value" : false
                },
               {
                     "name" : "Template1",
                     "type" : "template",
                     "value" : {
                         "isDefinition" : true,
                         "metrics" : [
                             { "name" : "myBool", "value" : false, "type" : "boolean" },
                             { "name" : "myInt", "value" : 0, "type" : "int" }
                         ],
                         "parameters" : [
                             {
                                 "name" : "param1",
                                 "type" : "string",
                                 "value" : "value1"
                             }
                         ]
                     }
                 }
             ]
         };
     },
 
     // Get BIRTH payload for the device
     getDeviceBirthPayload = function() {
         return {
             "timestamp" : new Date().getTime(),
             "metrics" : [
                 { "name" : "my_boolean", "value" : true, "type" : "boolean", "alias": 1 },
                 { "name" : "my_double", "value" : 1234567890.123456789, "type" : "double", "alias": 2 },
                 { "name" : "my_float", "value" : 12345.123456789, "type" : "float", "alias": 3 },
                 { "name" : "my_int", "value" : 1234567890, "type" : "int", "alias": 4 },
                 { "name" : "my_long", "value" : 123456789012345, "type" : "long","alias": 5 },
                 { "name" : "Inputs/0", "value" :  true, "type" : "boolean", "alias": 6 },
                 { "name" : "Inputs/1", "value" :  0, "type" : "int", "alias": 7 },
                 { "name" : "Inputs/2", "value" :  1.23, "type" : "float", "alias": 8 },
                 { "name" : "Outputs/0", "value" :  true, "type" : "boolean", "alias": 9 },
                 { "name" : "Outputs/1", "value" :  0, "type" : "int", "alias": 10 },
                 { "name" : "Outputs/2", "value" :  1.23, "type" : "float", "alias": 11 },
                 { "name" : "Properties/hw_version", "value" :  hwVersion, "type" : "string" },
                 { "name" : "Properties/sw_version", "value" :  swVersion, "type" : "string" },
                 { 
                     "name" : "my_dataset",
                     "type" : "dataset",
                     "value" : {
                         "numOfColumns" : 3,
                         "types" : [ "string", "string", 'long' ],
                         "columns" : [ "str1", "str2", 'lnum' ],
                         "rows" : [ 
                             [ "x", "a", 123456789012345 ],
                             [ "y", "b", 123456789012346 ]
                         ]
                     }
                 },
                 {
                     "name" : "TemplateInstance1",
                     "type" : "template",
                     "value" : {
                         "templateRef" : "Template1",
                         "isDefinition" : false,
                         "metrics" : [
                             { "name" : "myBool", "value" : true, "type" : "boolean" },
                             { "name" : "myInt", "value" : 100, "type" : "int" }
                         ],
                         "parameters" : [
                             {
                                 "name" : "param1",
                                 "type" : "string",
                                 "value" : "value2"
                             }
                         ]
                     }
                 }
             ]
         };
     },
     
     // Get data payload for the device
     getDataPayload = function() {
         return {
             "timestamp" : new Date().getTime(),
             "metrics" : [
                 { /*"name" : "my_boolean",*/ "alias": 1, "value" : Math.random() > 0.5, "type" : "boolean", "timestamp" : new Date().getTime() },
                 { /*"name" : "my_double",*/ "alias": 2, "value" : Math.random() * 0.123456789, "type" : "double" },
                 { /*"name" : "my_float",*/ "alias": 3, "value" : Math.random() * 0.123, "type" : "float" },
                 { /*"name" : "my_int",*/ "alias": 4, "value" : randomInt(), "type" : "int" },
                 { /*"name" : "my_long",*/ "alias": 5, "value" : randomInt() * 214748364700, "type" : "long", "timestamp" : new Date().getTime() }
             ]
         };
     },
     
     // Runs the sample
     run = function() {
         // Create the SparkplugClient
         sparkplugClient = SparkplugClient.newClient(config);
         
         sparkplugClient.logger.level = 'debug'

        sparkplugClient.on('connect', function(){
            let fname = 'digital-transformation.pdf';
            let data = Fs.readFileSync(fname);
            let payload = {
                "filename": fname,
                "data": data.toString('base64')
              }
              
            // properties are carried only for MQTT 5.0 connections
            
            // sparkplugClient.client.publish('C3ET/docs/bin/'+fname, Buffer.alloc(0), {qos: 0, retain: true});
            sparkplugClient.client.publish('C3ET/docs/bin/'+fname, data, {qos: 0, retain: true, properties: {payloadFormatIndicator: true, contentType: 'application/json'}});
            sparkplugClient.client.publish('C3ET/test/bool', 'true', {qos: 0, retain: true, properties: {payloadFormatIndicator: true, contentType: 'application/json'}});
            sparkplugClient.client.publish('C3ET/test/int', '12345', {qos: 0, retain: true, properties: {payloadFormatIndicator: true, contentType: 'application/json'}});
            sparkplugClient.client.publish('C3ET/test/float', '12345.6789', {qos: 0, retain: true, properties: {payloadFormatIndicator: true, contentType: 'application/json'}});
            sparkplugClient.client.publish('C3ET/test/string', 'a string 123', {qos: 0, retain: true, properties: {payloadFormatIndicator: true, contentType: 'text/plain'}});
            sparkplugClient.client.publish('C3ET/test/quotedstring', "'a string 123'", {qos: 0, retain: true, properties: {payloadFormatIndicator: true, contentType: 'application/json'}});
            sparkplugClient.client.publish('C3ET/test/jsonobj', "{'numberVar': 12345.2, 'boolVar': false }", {qos: 0, retain: true, properties: {payloadFormatIndicator: true, contentType: 'application/json'}});
            sparkplugClient.client.publish('C3ET/test/jsonarr', "[ 12345.2, 23456.7, 345678.9 ]", {qos: 0, retain: true, properties: {payloadFormatIndicator: true, contentType: 'application/json'}} );
        });

         // Create 'birth' handler
         sparkplugClient.on('birth', function () {
             // Publish Node BIRTH certificate
             sparkplugClient.publishNodeBirth(getNodeBirthPayload());
             // Publish Device BIRTH certificate
             sparkplugClient.publishDeviceBirth(deviceId, getDeviceBirthPayload());
         });
 
         // Create node command handler
         sparkplugClient.on('ncmd', function (payload) {
            console.log("Command received for node ");
            console.log(payload);

            let timestamp = payload.timestamp,
                 metrics = payload.metrics;
 
             if (metrics !== undefined && metrics !== null) {
                 for (let i = 0; i < metrics.length; i++) {
                     let metric = metrics[i];
                     if (metric.name == "Node Control/Rebirth" && metric.value) {
                         console.log("Received 'Rebirth' command");
                         // Publish Node BIRTH certificate
                         sparkplugClient.publishNodeBirth(getNodeBirthPayload());
                         // Publish Device BIRTH certificate
                         sparkplugClient.publishDeviceBirth(deviceId, getDeviceBirthPayload());
                     }
                     if (metric.name == "Node Control/Reboot" && metric.value) {
                        console.log("Received 'Reboot' command");
                        // Publish Device BIRTH certificate
                        sparkplugClient.publishDeviceDeath(deviceId, {});
                        // Publish Node BIRTH certificate
                        sparkplugClient.stop();
                        process.exit(1);
                    }
                 }
             }     
         });
         
         // Create device command handler
         sparkplugClient.on('dcmd', function (deviceId, payload) {
             let timestamp = payload.timestamp,
                 metrics = payload.metrics,
                 inboundMetricMap = {},
                 outboundMetric = [],
                 outboundPayload;
             
             console.log("Command received for device " + deviceId);
             
             // Loop over the metrics and store them in a map
             if (metrics !== undefined && metrics !== null) {
                 for (let i = 0; i < metrics.length; i++) {
                     let metric = metrics[i];
                     inboundMetricMap[metric.name] = metric.value;
                 }
             }
             if (inboundMetricMap["Outputs/0"] !== undefined && inboundMetricMap["Outputs/0"] !== null) {
                 console.log("Outputs/0: " + inboundMetricMap["Outputs/0"]);
                 outboundMetric.push({ "name" : "Inputs/0", "value" : inboundMetricMap["Outputs/0"], "type" : "boolean" });
                 outboundMetric.push({ "name" : "Outputs/0", "value" : inboundMetricMap["Outputs/0"], "type" : "boolean" });
                 console.log("Updated value for Inputs/0 " + inboundMetricMap["Outputs/0"]);
             } else if (inboundMetricMap["Outputs/1"] !== undefined && inboundMetricMap["Outputs/1"] !== null) {
                 console.log("Outputs/1: " + inboundMetricMap["Outputs/1"]);
                 outboundMetric.push({ "name" : "Inputs/1", "value" : inboundMetricMap["Outputs/1"], "type" : "int" });
                 outboundMetric.push({ "name" : "Outputs/1", "value" : inboundMetricMap["Outputs/1"], "type" : "int" });
                 console.log("Updated value for Inputs/1 " + inboundMetricMap["Outputs/1"]);
             } else if (inboundMetricMap["Outputs/2"] !== undefined && inboundMetricMap["Outputs/2"] !== null) {
                 console.log("Outputs/2: " + inboundMetricMap["Outputs/2"]);
                 outboundMetric.push({ "name" : "Inputs/2", "value" : inboundMetricMap["Outputs/2"], "type" : "float" });
                 outboundMetric.push({ "name" : "Outputs/2", "value" : inboundMetricMap["Outputs/2"], "type" : "float" });
                 console.log("Updated value for Inputs/2 " + inboundMetricMap["Outputs/2"]);
             }
 
             outboundPayload = {
                     "timestamp" : new Date().getTime(),
                     "metrics" : outboundMetric
             };
 
             // Publish device data
             sparkplugClient.publishDeviceData(deviceId, outboundPayload);             
         });
         
         for (let i = 1; i < 10001; i++) {
             // Set up a device data publish for i*publishPeriod milliseconds from now
             setTimeout(function() {
                 // Publish device data
                 sparkplugClient.publishDeviceData(deviceId, getDataPayload());

                 sparkplugClient.publishDeviceCmd("Sparkplug B Devices", "JSON-SCADA Server", "JSON-SCADA Device", {
                   timestamp: new Date().getTime(),
                   metrics: [
                       {
                           name: "command1",
                           value: true,
                           type: "boolean"
                       }
                   ]  
                 })
                 
                 // End the client connection after the last publish
                 if (i === 5000) {
                     sparkplugClient.stop();
                 }
             }, i*publishPeriod);
         }
     };
     
     return {run:run};
 }());
 
 // Run the sample
 sample.run();