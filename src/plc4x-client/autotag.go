/*
 * PLC4X Client - Generic PLC Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
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

package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RtDataTag struct {
	Id                             float64   `bson:"_id" json:"_id"`
	ProtocolSourceASDU             string    `bson:"protocolSourceASDU" json:"protocolSourceASDU"`
	ProtocolSourceCommonAddress    string    `bson:"protocolSourceCommonAddress" json:"protocolSourceCommonAddress"`
	ProtocolSourceConnectionNumber float64   `bson:"protocolSourceConnectionNumber" json:"protocolSourceConnectionNumber"`
	ProtocolSourceObjectAddress    string    `bson:"protocolSourceObjectAddress" json:"protocolSourceObjectAddress"`
	ProtocolSourceCommandUseSBO    bool      `bson:"protocolSourceCommandUseSBO" json:"protocolSourceCommandUseSBO"`
	ProtocolSourceCommandDuration  float64   `bson:"protocolSourceCommandDuration" json:"protocolSourceCommandDuration"`
	AlarmState                     float64   `bson:"alarmState" json:"alarmState"`
	AlarmRange                     float64   `bson:"alarmRange" json:"alarmRange"`
	Description                    string    `bson:"description" json:"description"`
	UngroupedDescription           string    `bson:"ungroupedDescription" json:"ungroupedDescription"`
	Group1                         string    `bson:"group1" json:"group1"`
	Group2                         string    `bson:"group2" json:"group2"`
	Group3                         string    `bson:"group3" json:"group3"`
	StateTextFalse                 string    `bson:"stateTextFalse" json:"stateTextFalse"`
	StateTextTrue                  string    `bson:"stateTextTrue" json:"stateTextTrue"`
	EventTextFalse                 string    `bson:"eventTextFalse" json:"eventTextFalse"`
	EventTextTrue                  string    `bson:"eventTextTrue" json:"eventTextTrue"`
	Origin                         string    `bson:"origin" json:"origin"`
	Tag                            string    `bson:"tag" json:"tag"`
	Type                           string    `bson:"type" json:"type"`
	Value                          float64   `bson:"value" json:"value"`
	ValueString                    string    `bson:"valueString" json:"valueString"`
	ValueJson                      bson.M    `bson:"valueJson" json:"valueJson"`
	AlarmDisabled                  bool      `bson:"alarmDisabled" json:"alarmDisabled"`
	Alerted                        bool      `bson:"alerted" json:"alerted"`
	Alarmed                        bool      `bson:"alarmed" json:"alarmed"`
	Annotation                     string    `bson:"annotation" json:"annotation"`
	CommandBlocked                 bool      `bson:"commandBlocked" json:"commandBlocked"`
	CommandOfSupervised            float64   `bson:"commandOfSupervised" json:"commandOfSupervised"`
	CommissioningRemarks           string    `bson:"commissioningRemarks" json:"commissioningRemarks"`
	Formula                        float64   `bson:"formula" json:"formula"`
	Frozen                         bool      `bson:"frozen" json:"frozen"`
	FrozenDetectTimeout            float64   `bson:"frozenDetectTimeout" json:"frozenDetectTimeout"`
	HiLimit                        float64   `bson:"hiLimit" json:"hiLimit"`
	HihiLimit                      float64   `bson:"hihiLimit" json:"hihiLimit"`
	HihihiLimit                    float64   `bson:"hihihiLimit" json:"hihihiLimit"`
	HistorianDeadBand              float64   `bson:"historianDeadBand" json:"historianDeadBand"`
	HistorianPeriod                float64   `bson:"historianPeriod" json:"historianPeriod"`
	Hysteresis                     float64   `bson:"hysteresis" json:"hysteresis"`
	Invalid                        bool      `bson:"invalid" json:"invalid"`
	InvalidDetectTimeout           float64   `bson:"invalidDetectTimeout" json:"invalidDetectTimeout"`
	IsEvent                        bool      `bson:"isEvent" json:"isEvent"`
	Kconv1                         float64   `bson:"kconv1" json:"kconv1"`
	Kconv2                         float64   `bson:"kconv2" json:"kconv2"`
	Location                       bson.M    `bson:"location" json:"location"`
	LoLimit                        float64   `bson:"loLimit" json:"loLimit"`
	LoloLimit                      float64   `bson:"loloLimit" json:"loloLimit"`
	LololoLimit                    float64   `bson:"lololoLimit" json:"lololoLimit"`
	Notes                          string    `bson:"notes" json:"notes"`
	Overflow                       bool      `bson:"overflow" json:"overflow"`
	Parcels                        []float64 `bson:"parcels" json:"parcels"`
	Priority                       float64   `bson:"priority" json:"priority"`
	ProtocolDestinations           bson.M    `bson:"protocolDestinations" json:"protocolDestinations"`
	SourceDataUpdate               bson.M    `bson:"sourceDataUpdate" json:"sourceDataUpdate"`
	SupervisedOfCommand            float64   `bson:"supervisedOfCommand" json:"supervisedOfCommand"`
	Substituted                    bool      `bson:"substituted" json:"substituted"`
	TimeTag                        time.Time `bson:"timeTag" json:"timeTag"`
	TimeTagAlarm                   time.Time `bson:"timeTagAlarm" json:"timeTagAlarm"`
	TimeTagAtSource                time.Time `bson:"timeTagAtSource" json:"timeTagAtSource"`
	TimeTagAtSourceOk              time.Time `bson:"timeTagAtSourceOk" json:"timeTagAtSourceOk"`
	Transient                      bool      `bson:"transient" json:"transient"`
	Unit                           string    `bson:"unit" json:"unit"`
	UpdatesCnt                     float64   `bson:"updatesCnt" json:"updatesCnt"`
	ValueDefault                   float64   `bson:"valueDefault" json:"valueDefault"`
	ZeroDeadband                   float64   `bson:"zeroDeadband" json:"zeroDeadband"`
}

const _AutoKeyMultiplier = 1000000 // should be more than estimated maximum points on a connection
var _AutoKeyId = 0
var _ListCreatedTags map[string]string = map[string]string{}

func NewRtDataTag() RtDataTag {
	_AutoKeyId++
	return RtDataTag{
		Id:          float64(_AutoKeyId),
		AlarmState:  -1,
		Origin:      "supervised",
		Type:        "analog",
		Group1:      "",
		Group2:      "",
		Group3:      "",
		HiLimit:     math.MaxFloat64,
		HihiLimit:   math.MaxFloat64,
		HihihiLimit: math.MaxFloat64,
		LoLimit:     -math.MaxFloat64,
		LoloLimit:   -math.MaxFloat64,
		LololoLimit: -math.MaxFloat64,
		Kconv1:      1,
	}
}

func (pc *protocolConnection) GetAutoKeyInitialValueConn(collectionRtData *mongo.Collection, connectionNumber int) int {
	pc.AutoKeyId = connectionNumber * _AutoKeyMultiplier

	// read connections config
	filter := bson.D{
		{Key: "_id", Value: bson.D{
			{Key: "$gt", Value: _AutoKeyId},
			{Key: "$lt", Value: (connectionNumber + 1) * _AutoKeyMultiplier},
		},
		},
	}
	var res bson.M
	opts := options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})
	err := collectionRtData.FindOne(context.TODO(), filter, opts).Decode(&res)
	if err != nil {
		v, ok := res["_id"].(float64)
		if ok && v > float64(_AutoKeyId) {
			pc.AutoKeyId = int(v)
		}
	}
	return pc.AutoKeyId
}

func (pc *protocolConnection) AutoCreateTag(rtData *RtDataTag, rtDataCollection *mongo.Collection) {
	if rtData.Tag == "" {
		rtData.Tag = fmt.Sprintf("%s.%d.%s", DriverName, int(rtData.ProtocolSourceConnectionNumber), rtData.ProtocolSourceObjectAddress)
	}

	if _, ok := _ListCreatedTags[rtData.Tag]; ok { // if already in the created list, just returns
		return
	}

	// not in the list, so try to find it, will create a new tag if not found
	filter := bson.D{
		{Key: "protocolSourceObjectAddress", Value: rtData.ProtocolSourceObjectAddress},
		{Key: "protocolSourceConnectionNumber", Value: rtData.ProtocolSourceConnectionNumber},
	}
	var res bson.M
	err := rtDataCollection.FindOne(context.TODO(), filter).Decode(&res)
	if err == nil {
		_, ok := res["_id"].(float64)
		if ok {
			_ListCreatedTags[rtData.Tag] = res["type"].(string)
		}
		return
	}

	// not found, so insert a new tag
	pc.AutoKeyId++
	rtData.Id = float64(pc.AutoKeyId)
	rtData.Description = rtData.Tag
	rtData.UngroupedDescription = rtData.ProtocolSourceObjectAddress
	rtData.EventTextFalse = "OFF"
	rtData.EventTextTrue = "ON"
	rtData.StateTextFalse = "OFF"
	rtData.StateTextTrue = "ON"
	rtData.Origin = "supervised"
	rtData.Unit = ""
	if _, err := rtDataCollection.InsertOne(context.TODO(), rtData); err != nil {
		log.Println("Mongodb: Error inserting tag", rtData.Tag, err)
	}
	_ListCreatedTags[rtData.Tag] = rtData.Type
}
