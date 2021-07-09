'use strict'

/*
 * Updater for SAGE (Cepel) HTML/SVG Displays (SAGE-web)
 *
 * {json:scada} - Copyright (c) 2020-2021 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

var updatePeriod = 3000
var statusInvalidColor = 'white'
var sageDisplayDir = '/sage-cepel-displays'
function paddingLeft (str, paddingValue) {
  return String(paddingValue + str).slice(-paddingValue.length)
}
function countDecimals (str) {
  var a = str.split('.')
  if (a.length <= 1) return 0
  return a[1].length || 0
}
function addCSS (url) {
  var head = document.getElementsByTagName('head')[0]
  var link = document.createElement('link')
  // link.id   = cssId;
  link.rel = 'stylesheet'
  link.type = 'text/css'
  link.href = url
  link.media = 'all'
  head.appendChild(link)
}
function addScript (url) {
  var script = document.createElement('script')
  script.setAttribute('src', url)
  document.head.appendChild(script)
}
addCSS(sageDisplayDir + '/css/base.css')
addCSS(sageDisplayDir + '/css/font.css')
addCSS(sageDisplayDir + '/css/form.css')
addScript(sageDisplayDir + '/scripts/main.js')
addScript('/opc-codes.js')
addScript('/util.js')

var queryKeys = []
var keysIdsMap = []
var msgSage = {}

document.addEventListener('DOMContentLoaded', function () {
  console.log('DOMContentLoaded')

  // socket_di = null
  var nodeList = document.querySelectorAll("[id^='ID']")
  var pos
  nodeList.forEach(node => {
    if (node.parentNode.tagName === 'a')
      if ('baseVal' in node.parentNode.href) {
        pos = node.parentNode.href.baseVal.indexOf('&id=')
        if (pos > 0) {
          var sageType = node.getAttribute('sage:tipo')
          switch (sageType) {
            case 'Medida':
              node.OriginalFill = node.style.fill
              node.style.fill = statusInvalidColor
              break
            case 'Disjuntor':
              node.style.stroke = statusInvalidColor
              node.style.fill = statusInvalidColor
              break
            case 'Seccionadora':
              node.style.stroke = statusInvalidColor
              break
          }

          var key = node.parentNode.href.baseVal.substring(pos + 4)
          //key = key.replace('LAJ2', 'KAW2')
          if (!(key in keysIdsMap)) keysIdsMap[key] = {}
          if (!('listIds' in keysIdsMap[key])) keysIdsMap[key].listIds = []
          if (node.tagName === 'text') {
            if (!('decimals' in keysIdsMap[key]))
              keysIdsMap[key].decimals = countDecimals(node.textContent)
            if (!('padding' in keysIdsMap[key]))
              keysIdsMap[key].padding = node.textContent
                .replace('.', ' ')
                .replace(/0/g, ' ')
          } else {
            if (!('decimals' in keysIdsMap[key])) keysIdsMap[key].decimals = 5
            if (!('padding' in keysIdsMap[key])) keysIdsMap[key].padding = ''
          }
          keysIdsMap[key].listIds.push(node.id)
          msgSage[node.id] = {}

          if (!queryKeys.includes(key)) queryKeys.push(key)

          //console.log(key)
          //console.log(queryKeys[key])
        }
      }
  })

  var nodeListA = document.querySelectorAll('a[href*="html"]')
  nodeListA.forEach(node => {
    var path = new URL(node.href).pathname
    if (path.indexOf('/') === 0) node.href = sageDisplayDir + path
    // console.log( (new URL(node.href)).pathname )
  })

  setTimeout(() => {
    //console.log('Timer')

    // use this to redefine failed values color
    corRgalr = function (val) {
      return statusInvalidColor
    }

    setInterval(() => {
      getRealtimeData(queryKeys, false, () => {
        //console.log('loaded')

        //console.log(msgSage)
        socket_di.onmessage({ data: JSON.stringify(msgSage) })
      })
    }, updatePeriod)

    getRealtimeData(queryKeys, true, () => {
      //console.log('loaded')

      //console.log(msgSage)
      socket_di.onmessage({ data: JSON.stringify(msgSage) })
    })
  }, 1000)
})

// obtains realtime data from the server translating it to
// internal data structures then call the SVG screen update routine
function getRealtimeData (querykeys, askinfo, callbacksuccess) {
  // use OPC web hmi protocol https://prototyping.opcfoundation.org/
  var ServiceId = OpcServiceCode.ReadRequest // read data service
  var RequestHandle = Math.floor(Math.random() * 100000000)
  var req = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 1500,
        ReturnDiagnostics: 2,
        AuthenticationToken: null
      },
      MaxAge: 0,
      TimestampsToReturn: TimestampsToReturn.Both
    }
  }

  var NodesToRead = []
  querykeys.map(element => {
    var IdType, Id
    if (isNaN(parseInt(element))) {
      // a tag
      IdType = OpcKeyType.String
      Id = element
    } else {
      // a numeric key
      IdType = OpcKeyType.Numeric
      Id = parseInt(element)
    }
    NodesToRead.push({
      NodeId: {
        IdType: IdType,
        Id: Id,
        Namespace: OpcNamespaceMongodb
      },
      AttributeId:
        askinfo === true ? OpcAttributeId.Description : OpcAttributeId.Value
    })
    return element
  })
  req.Body.NodesToRead = NodesToRead

  fetchTimeout('/Invoke/', 1500, {
    method: 'POST',
    body: JSON.stringify(req),
    headers: {
      'Content-Type': 'application/json'
    }
  })
    .then(function (response) {
      return response
    })
    .then(response => response.json())
    .then(data => {
      if (
        !data.ServiceId ||
        !data.Body ||
        !data.Body.ResponseHeader ||
        !data.Body.ResponseHeader.RequestHandle
      ) {
        console.log('ReadRequest invalid service response!')
        return
      }

      // response must have same request handle and be a read response or service fault
      if (
        data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
        (data.ServiceId !== OpcServiceCode.ReadResponse &&
          data.ServiceId !== OpcServiceCode.ServiceFault)
      ) {
        console.log('ReadRequest invalid or unexpected service response!')
        return
      }

      if (
        data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
        data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
      ) {
        console.log('ReadRequest service error!')
        // check access control denied, in this case go to initial page
        if (
          data.Body.ResponseHeader.ServiceResult ===
            OpcStatusCodes.BadUserAccessDenied ||
          data.Body.ResponseHeader.ServiceResult ===
            OpcStatusCodes.BadIdentityTokenInvalid ||
          data.Body.ResponseHeader.ServiceResult ===
            OpcStatusCodes.BadIdentityTokenRejected
        ) {
          window.onbeforeunload = null
          window.location.href = '/'
        }
        return
      }

      if ('Results' in data.Body)
        data.Body.Results.map(element => {
          if (typeof element.StatusCode == 'number' && element.StatusCode != 0)
            // reject a bad result
            return
          if (element.NodeId.IdType != 1) return

          //console.log(element)
          keysIdsMap[element.NodeId.Id].listIds.forEach(k => {
            if (element.Value.Type === 1) {
              msgSage[k] = { val: element.Value.Body === true ? 1 : 2 }
              //msgSage[k].rgalr = 0
            }

            if (element.Value.Type === 11) {
              msgSage[k] = {
                val: paddingLeft(
                  parseFloat(element.Value.Body)
                    .toFixed(keysIdsMap[element.NodeId.Id].decimals)
                    .toString(),
                  keysIdsMap[element.NodeId.Id].padding
                )
              }
            }

            var svgElement = document.getElementById(k)
            var sageType = svgElement.getAttribute('sage:tipo')
            if (element.Value.Quality !== 0) {
              switch (sageType) {
                case 'Medida':
                  msgSage[k].rgalr = 0
                  break
                case 'Disjuntor':
                  console.log(k)
                  if (element.Value.Body)
                    setTimeout(() => {
                      svgElement.style.stroke = statusInvalidColor
                      svgElement.style.fill = statusInvalidColor
                    }, 1)
                  else
                    setTimeout(() => {
                      svgElement.style.stroke = statusInvalidColor
                    }, 1)
                  break
                case 'Seccionadora':
                  setTimeout(() => {
                    svgElement.style.stroke = statusInvalidColor
                  }, 1)
                  break
              }
            } else {
              if (sageType === 'Medida')
                svgElement.style.fill = svgElement.OriginalFill
            }
          })

          return element
        })
      if (typeof callbacksuccess == 'function') callbacksuccess()
    })
    .catch(function (error) {
      console.log(error)
    })
}
