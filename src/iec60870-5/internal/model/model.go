/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - shared data model
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
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

package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// DriverInstance mirrors a protocolDriverInstances document.
type DriverInstance struct {
	ID                            bson.ObjectID `bson:"_id"`
	ProtocolDriver                string        `bson:"protocolDriver"`
	ProtocolDriverInstanceNumber  float64       `bson:"protocolDriverInstanceNumber"`
	Enabled                       bool          `bson:"enabled"`
	LogLevel                      float64       `bson:"logLevel"`
	NodeNames                     []string      `bson:"nodeNames"`
	ActiveNodeName                string        `bson:"activeNodeName"`
	ActiveNodeKeepAliveTimeTag    time.Time     `bson:"activeNodeKeepAliveTimeTag"`
	KeepProtocolRunningWhileInact bool          `bson:"keepProtocolRunningWhileInactive"`
}

// ConnCfg mirrors a protocolConnections document; superset of the fields
// used by the four IEC 60870-5 drivers. Numeric fields are float64 so any
// BSON numeric type decodes (parity with the C# permissive serializers).
type ConnCfg struct {
	ID                            bson.ObjectID `bson:"_id"`
	ProtocolDriver                string        `bson:"protocolDriver"`
	ProtocolDriverInstanceNumber  float64       `bson:"protocolDriverInstanceNumber"`
	ProtocolConnectionNumber      float64       `bson:"protocolConnectionNumber"`
	Name                          string        `bson:"name"`
	Description                   string        `bson:"description"`
	Enabled                       bool          `bson:"enabled"`
	CommandsEnabled               *bool         `bson:"commandsEnabled"` // default true when missing
	IPAddressLocalBind            string        `bson:"ipAddressLocalBind"`
	IPAddresses                   []string      `bson:"ipAddresses"`
	LocalLinkAddress              float64       `bson:"localLinkAddress"`
	RemoteLinkAddress             float64       `bson:"remoteLinkAddress"`
	GiInterval                    *float64      `bson:"giInterval"` // default 300 when missing
	TestCommandInterval           float64       `bson:"testCommandInterval"`
	TimeSyncInterval              float64       `bson:"timeSyncInterval"`
	SizeOfCOT                     float64       `bson:"sizeOfCOT"`
	SizeOfCA                      float64       `bson:"sizeOfCA"`
	SizeOfIOA                     float64       `bson:"sizeOfIOA"`
	K                             float64       `bson:"k"`
	W                             float64       `bson:"w"`
	T0                            float64       `bson:"t0"`
	T1                            float64       `bson:"t1"`
	T2                            float64       `bson:"t2"`
	T3                            float64       `bson:"t3"`
	ServerModeMultiActive         *bool         `bson:"serverModeMultiActive"` // default true when missing
	MaxClientConnections          float64       `bson:"maxClientConnections"`
	MaxQueueSize                  float64       `bson:"maxQueueSize"`
	LocalCertFilePath             string        `bson:"localCertFilePath"`
	Passphrase                    string        `bson:"passphrase"`
	PeerCertFilesPaths            []string      `bson:"peerCertFilesPaths"`
	RootCertFilePath              string        `bson:"rootCertFilePath"`
	AllowOnlySpecificCertificates bool          `bson:"allowOnlySpecificCertificates"`
	ChainValidation               bool          `bson:"chainValidation"`
	AutoCreateTags                bool          `bson:"autoCreateTags"`
	Topics                        []string      `bson:"topics"`
	PeerIdentification            *bool         `bson:"peerIdentification"` // default true when missing
	Stats                         bson.M        `bson:"stats"`              // the stored statistics (stats.peerIdentity)
	// IEC 101 (serial) specific
	PortName          string  `bson:"portName"`
	BaudRate          float64 `bson:"baudRate"`
	Parity            string  `bson:"parity"`
	StopBits          string  `bson:"stopBits"`
	Handshake         string  `bson:"handshake"`
	TimeoutForACK     float64 `bson:"timeoutForACK"`
	TimeoutRepeat     float64 `bson:"timeoutRepeat"`
	TimeoutMessage    float64 `bson:"timeoutMessage"`
	TimeoutCharacter  float64 `bson:"timeoutCharacter"`
	UseSingleCharACK  bool    `bson:"useSingleCharACK"`
	SizeOfLinkAddress float64 `bson:"sizeOfLinkAddress"`
}

// CommandsEnabledVal returns commandsEnabled with the C# default (true).
func (c *ConnCfg) CommandsEnabledVal() bool {
	if c.CommandsEnabled == nil {
		return true
	}
	return *c.CommandsEnabled
}

// PeerIdentificationVal returns peerIdentification (default true): identify
// the peer on every connection into stats.peerIdentity and report changes of
// its identity as security events.
func (c *ConnCfg) PeerIdentificationVal() bool {
	if c.PeerIdentification == nil {
		return true
	}
	return *c.PeerIdentification
}

// GiIntervalVal returns giInterval with the C# default (300 s).
func (c *ConnCfg) GiIntervalVal() int {
	if c.GiInterval == nil {
		return 300
	}
	return int(*c.GiInterval)
}

// MultiActiveVal returns serverModeMultiActive with the C# default (true).
func (c *ConnCfg) MultiActiveVal() bool {
	if c.ServerModeMultiActive == nil {
		return true
	}
	return *c.ServerModeMultiActive
}

// ProtocolDestination mirrors one entry of realtimeData.protocolDestinations.
// The address fields are U32: they accept a string or any numeric BSON type.
type ProtocolDestination struct {
	ConnectionNumber float64 `bson:"protocolDestinationConnectionNumber"`
	CommonAddress    U32     `bson:"protocolDestinationCommonAddress"`
	ObjectAddress    U32     `bson:"protocolDestinationObjectAddress"`
	ASDU             U32     `bson:"protocolDestinationASDU"`
	CommandDuration  float64 `bson:"protocolDestinationCommandDuration"`
	CommandUseSBO    bool    `bson:"protocolDestinationCommandUseSBO"`
	Group            float64 `bson:"protocolDestinationGroup"`
	KConv1           float64 `bson:"protocolDestinationKConv1"`
	KConv2           float64 `bson:"protocolDestinationKConv2"`
	HoursShift       float64 `bson:"protocolDestinationHoursShift"`
}

// RtDataPoint is the projection of a realtimeData document needed by the
// server drivers (interrogation, read and command handling).
// The protocol source address fields are U32: they accept a string or any
// numeric BSON type.
type RtDataPoint struct {
	ID                             float64               `bson:"_id"`
	Tag                            string                `bson:"tag"`
	Value                          float64               `bson:"value"`
	ValueString                    string                `bson:"valueString"`
	Invalid                        bool                  `bson:"invalid"`
	Transient                      bool                  `bson:"transient"`
	Substituted                    bool                  `bson:"substituted"`
	Overflow                       bool                  `bson:"overflow"`
	Origin                         string                `bson:"origin"`
	Type                           string                `bson:"type"`
	TimeTagAtSource                *time.Time            `bson:"timeTagAtSource"`
	TimeTagAtSourceOk              *bool                 `bson:"timeTagAtSourceOk"`
	KConv1                         float64               `bson:"kconv1"`
	KConv2                         float64               `bson:"kconv2"`
	ProtocolSourceConnectionNumber float64               `bson:"protocolSourceConnectionNumber"`
	ProtocolSourceCommonAddress    U32                   `bson:"protocolSourceCommonAddress"`
	ProtocolSourceObjectAddress    U32                   `bson:"protocolSourceObjectAddress"`
	ProtocolSourceASDU             U32                   `bson:"protocolSourceASDU"`
	ProtocolSourceCommandDuration  float64               `bson:"protocolSourceCommandDuration"`
	ProtocolSourceCommandUseSBO    bool                  `bson:"protocolSourceCommandUseSBO"`
	ProtocolDestinations           []ProtocolDestination `bson:"protocolDestinations"`
}
