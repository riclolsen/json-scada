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
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RtDataTag struct {
	Id                             float64   `json:"_id"`
	ProtocolSourceASDU             string    `json:"protocolSourceASDU"`
	ProtocolSourceCommonAddress    string    `json:"protocolSourceCommonAddress"`
	ProtocolSourceConnectionNumber float64   `json:"protocolSourceConnectionNumber"`
	ProtocolSourceObjectAddress    string    `json:"protocolSourceObjectAddress"`
	ProtocolSourceCommandUseSBO    bool      `json:"protocolSourceCommandUseSBO"`
	ProtocolSourceCommandDuration  float64   `json:"protocolSourceCommandDuration"`
	AlarmState                     float64   `json:"alarmState"`
	AlarmRange                     float64   `json:"alarmRange"`
	Description                    string    `json:"description"`
	UngroupedDescription           string    `json:"ungroupedDescription"`
	Group1                         string    `json:"group1"`
	Group2                         string    `json:"group2"`
	Group3                         string    `json:"group3"`
	StateTextFalse                 string    `json:"stateTextFalse"`
	StateTextTrue                  string    `json:"stateTextTrue"`
	EventTextFalse                 string    `json:"eventTextFalse"`
	EventTextTrue                  string    `json:"eventTextTrue"`
	Origin                         string    `json:"origin"`
	Tag                            string    `json:"tag"`
	Type                           string    `json:"type"`
	Value                          float64   `json:"value"`
	ValueString                    string    `json:"valueString"`
	ValueJson                      bson.M    `json:"valueJson"`
	AlarmDisabled                  bool      `json:"alarmDisabled"`
	Alerted                        bool      `json:"alerted"`
	Alarmed                        bool      `json:"alarmed"`
	Annotation                     string    `json:"annotation"`
	CommandBlocked                 bool      `json:"commandBlocked"`
	CommandOfSupervised            float64   `json:"commandOfSupervised"`
	CommissioningRemarks           string    `json:"commissioningRemarks"`
	Formula                        float64   `json:"formula"`
	Frozen                         bool      `json:"frozen"`
	FrozenDetectTimeout            float64   `json:"frozenDetectTimeout"`
	HiLimit                        float64   `json:"hiLimit"`
	HihiLimit                      float64   `json:"hihiLimit"`
	HihihiLimit                    float64   `json:"hihihiLimit"`
	HistorianDeadBand              float64   `json:"historianDeadBand"`
	HistorianPeriod                float64   `json:"historianPeriod"`
	Hysteresis                     float64   `json:"hysteresis"`
	Invalid                        bool      `json:"invalid"`
	InvalidDetectTimeout           float64   `json:"invalidDetectTimeout"`
	IsEvent                        bool      `json:"isEvent"`
	Kconv1                         float64   `json:"kconv1"`
	Kconv2                         float64   `json:"kconv2"`
	Location                       bson.M    `json:"location"`
	LoLimit                        float64   `json:"loLimit"`
	LoloLimit                      float64   `json:"loloLimit"`
	LololoLimit                    float64   `json:"lololoLimit"`
	Notes                          string    `json:"notes"`
	Overflow                       bool      `json:"overflow"`
	Parcels                        []float64 `json:"parcels"`
	Priority                       float64   `json:"priority"`
	ProtocolDestinations           bson.M    `json:"protocolDestinations"`
	SourceDataUpdate               bson.M    `json:"sourceDataUpdate"`
	SupervisedOfCommand            float64   `json:"supervisedOfCommand"`
	Substituted                    bool      `json:"substituted"`
	TimeTag                        time.Time `json:"timeTag"`
	TimeTagAlarm                   time.Time `json:"timeTagAlarm"`
	TimeTagAtSource                time.Time `json:"timeTagAtSource"`
	TimeTagAtSourceOk              time.Time `json:"timeTagAtSourceOk"`
	Transient                      bool      `json:"transient"`
	Unit                           string    `json:"unit"`
	UpdatesCnt                     float64   `json:"updatesCnt"`
	ValueDefault                   float64   `json:"valueDefault"`
	ZeroDeadband                   float64   `json:"zeroDeadband"`
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
	rtDataCollection.InsertOne(context.TODO(), rtData)
	_ListCreatedTags[rtData.Tag] = rtData.Type
}
