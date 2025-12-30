const { ApolloServer } = require('@apollo/server')
const { expressMiddleware } = require('@apollo/server/express4')
const cors = require('cors')
const express = require('express')
const { authJwt } = require('./app/middlewares')

async function initGQLServer(app, db) {
  // Create an Apollo server
  const srvApollo = new ApolloServer({
    typeDefs: `#graphql
          type Query {
            getUsers: [User!]!
            getUserByName(username: String!): User
          }
  
          type User {
            _id: ID!
            username: String!
            email: String
            roles: [ID!]
          }
  
          type Query {
            getTagsByGroup1(group1: String!): [Tag!]!
            getTags(tags: [String]!): [Tag!]!
            getTag(tag: String!): Tag
            getProtocolDriverInstances: [ProtocolDriverInstance!]!
            getProtocolConnections: [ProtocolConnection!]!
            }
          
          type SourceDataUpdate {
            valueAtSource: Float
            valueStringAtSource: String
            valueJsonAtSource: String
            asduAtSource: String
            causeOfTransmissionAtSource: String
            timeTagAtSource: Float
            timeTagAtSourceOk: Boolean
            timeTag: Float
            notTopicalAtSource: Boolean
            invalidAtSource: Boolean
            overflowAtSource: Boolean
            blockedAtSource: Boolean
            substitutedAtSource: Boolean
            originator: String
          }
            
          type Tag {
            _id: Float!
            tag: String!
            value: Float
            valueString: String
            valueJson: String
            alarmDisabled: Boolean
            alarmRange: Float
            alarmState: Float
            alarmed: Boolean
            alerted: Boolean
            alertState: String
            annotation: String
            commandBlocked: Boolean
            commandOfSupervised: Float
            commissioningRemarks: String
            description: String
            eventTextFalse: String
            eventTextTrue: String
            formula: Float
            frozen: Boolean
            frozenDetectTimeout: Float
            group1: String
            group2: String
            group3: String
            hihihiLimit: Float
            hihiLimit: Float
            hiLimit: Float
            historianDeadBand: Float
            historianPeriod: Float
            historianLastValue: Float
            hysteresis: Float
            invalid: Boolean
            invalidDetectTimeout: Float
            isEvent: Boolean
            kconv1: Float
            kconv2: Float
            loLimit: Float
            loloLimit: Float
            lololoLimit: Float
            notes: String
            origin: String
            overflow: Boolean
            priority: Float
            protocolSourceConnectionNumber: Float
            protocolSourceObjectAddress: String
            stateTextFalse: String
            stateTextTrue: String
            substituted: Boolean
            supervisedOfCommand: Float
            timeTag:Float
            timeTagAlarm: Float
            timeTagAlertState: Float
            timeTagAtSource: Float
            timeTagAtSourceOk: Boolean
            transient: Boolean
            type: String
            ungroupedDescription: String
            unit: String
            updatesCnt: Float
            zeroDeadband: Float
            sourceDataUpdate: SourceDataUpdate
            }
          
          type ProtocolDriverInstance {
            _id: ID!
            protocolDriver: String!
            protocolDriverInstanceNumber: Float!
            enabled: Boolean
            logLevel: Float
            nodeNames: [String]
            keepProtocolRunningWhileInactive: Boolean
            activeNodeName: String
            activeNodeKeepAliveTimeTag: Float
            softwareVersion: String
            }

          type ProtocolConnection {
            _id: ID!
            protocolDriver: String!
            protocolDriverInstanceNumber: Float
            protocolConnectionNumber: Float
            name: String
            description: String
            enabled: Boolean
            commandsEnabled: Boolean
          }  
        `,
    resolvers: {
      Query: {
        getUsers: async () => {
          return await db.user.find()
        },
        getUserByName: async (_, qry) => {
          return await db.user.findOne({ username: qry.username })
        },
        getTagsByGroup1: async (_, qry) => {
          return await db.tag.find({group1: qry.group1})
        },
        getTags: async (_, qry) => {
          return await db.tag.find({tag: {$in : qry.tags}})
        },
        getTag: async (_, qry) => {
          return await db.tag.findOne({ tag: qry.tag })
        },
        getProtocolDriverInstances: async () => {
          return await db.protocolDriverInstance.find()
        },
        getProtocolConnections: async () => {
          return await db.protocolConnection.find()
        },
      },
    },
  })
  await srvApollo.start()
  app.use(cors(), express.json())
  app.use('/apollo', [authJwt.verifyToken], expressMiddleware(srvApollo))
}

module.exports = initGQLServer
