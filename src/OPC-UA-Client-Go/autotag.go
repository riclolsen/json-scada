/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
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

// Automatic tag creation. Port of the autoCreateTags block of
// ConsoleClient() in AsduReceiveHandler.cs: read the attributes and values
// of everything browsing found, queue the values so the writer creates the
// tags, and register the variables for monitoring.

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

// maxNodesToRead is the C# batch size for the attribute and value reads.
const maxNodesToRead = 500

// minValueReadChunk is the smallest value-read size worth trying before
// giving up on shrinking.
const minValueReadChunk = 10

// deviation D19: errValueReadTooLarge marks a discovery attempt that died inside a value
// read. The connection loop shrinks the read size before trying again.
var errValueReadTooLarge = errors.New("the server closed the connection during a value read")

// nodeAttrs are the attributes the read pass needs. NodeClass comes first
// because its status stands in for C#'s per-node ServiceResult: a node
// whose class cannot be read is skipped.
var nodeAttrs = []ua.AttributeID{
	ua.AttributeIDNodeClass,
	ua.AttributeIDBrowseName,
	ua.AttributeIDDisplayName,
	ua.AttributeIDUserAccessLevel,
	ua.AttributeIDExecutable,
	ua.AttributeIDUserExecutable,
}

// nodeInfo is one node as the read pass sees it.
type nodeInfo struct {
	NodeID         *ua.NodeID
	Address        string
	OK             bool
	NodeClass      ua.NodeClass
	BrowseName     string
	DisplayName    string
	AccessLevel    byte
	Executable     bool
	UserExecutable bool
}

// sessionLost reports whether an error means the session is gone rather
// than the individual read having failed. Discovery cannot continue over a
// dead session: every later batch would fail instantly and the connection
// would end up subscribing a truncated set of points.
func sessionLost(cli *opcua.Client, err error) bool {
	if cli.State() != opcua.Connected {
		return true
	}
	var code ua.StatusCode
	if errors.As(err, &code) {
		switch code {
		case ua.StatusBadServerNotConnected, ua.StatusBadSessionIDInvalid,
			ua.StatusBadSessionClosed, ua.StatusBadSecureChannelClosed,
			ua.StatusBadSecureChannelIDInvalid, ua.StatusBadConnectionClosed,
			ua.StatusBadTCPMessageTooLarge, ua.StatusBadCommunicationError:
			return true
		}
	}
	return false
}

// autotagPass discovers tags from a browsed address space.
//
// It returns an error when the session was lost partway through. The caller
// must then rebuild the connection and browse again: a partial pass would
// otherwise leave most of the namespace untagged until the driver is
// restarted.
func autotagPass(ctx context.Context, cli *opcua.Client, conn *OPCUAConnection, browsed *browseResult) error {
	jslog.Log(jslog.LevelNoLog, "%s - Browsing the OPC UA server namespace.", conn.Name)

	// parity: the topic filter runs twice with two different rules. This is
	// the first: a topic must match a whole path segment.
	if len(conn.Topics) > 0 {
		jslog.Log(jslog.LevelNoLog, "%s - Filtering nodes by topics: %s", conn.Name, strings.Join(conn.Topics, ", "))
		kept := browsed.Order[:0]
		for _, key := range browsed.Order {
			e := browsed.Refs[key]
			if e.Ref.NodeClass == ua.NodeClassObject {
				kept = append(kept, key)
				continue
			}
			if e.Ref.NodeClass == ua.NodeClassMethod && !conn.CommandsEnabled {
				kept = append(kept, key)
				continue
			}
			keep := false
			for _, topic := range conn.Topics {
				if strings.Contains(e.Path+"/", "/"+topic+"/") {
					keep = true
					break
				}
			}
			if keep {
				kept = append(kept, key)
			} else {
				delete(browsed.Refs, key)
			}
		}
		browsed.Order = kept
	}
	jslog.Log(jslog.LevelNoLog, "%s - %d nodes found in the namespace.", conn.Name, len(browsed.Refs))

	// Objects are containers, not data; methods only matter when commands
	// are allowed.
	var toRead, metaKeys []string
	for _, key := range browsed.Order {
		e, ok := browsed.Refs[key]
		if !ok {
			continue
		}
		if e.Ref.NodeClass == ua.NodeClassObject {
			continue
		}
		// deviation D39: a variable's unit and range describe it; they get
		// no tag of their own.
		if isMetadataProperty(e) {
			metaKeys = append(metaKeys, key)
			continue
		}
		if e.Ref.NodeClass == ua.NodeClassMethod && !conn.CommandsEnabled {
			continue
		}
		toRead = append(toRead, key)
	}

	// deviation D31: a node whose value reads null is registered for
	// monitoring but never gets a tag, so a later pass on a rebuilt session
	// would register it again. Remember what is already monitored.
	monitored := make(map[string]bool, len(conn.ListMon))
	for _, it := range conn.ListMon {
		monitored[it.NodeID] = true
	}

	maxPerRead := operationLimit(ctx, cli, id.Server_ServerCapabilities_OperationLimits_MaxNodesPerRead)

	meta, err := readEngMetadata(ctx, cli, conn, browsed, metaKeys, maxPerRead)
	if err != nil {
		if sessionLost(cli, err) {
			return fmt.Errorf("%w: reading %d unit and range properties %d at a time: %v",
				errValueReadTooLarge, len(metaKeys), conn.ValueReadChunk(), err)
		}
		// Only the unit and limits of the new tags are missing.
		jslog.Log(jslog.LevelBasic, "%s - Error reading unit and range properties: %v", conn.Name, err)
	}

	for offset := 0; offset < len(toRead); offset += maxNodesToRead {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		end := min(offset+maxNodesToRead, len(toRead))
		batch := toRead[offset:end]

		infos, err := readNodeAttributes(ctx, cli, batch, maxPerRead)
		if err != nil && !sessionLost(cli, err) && ctx.Err() == nil {
			// One more try before giving up on the batch.
			jslog.Log(jslog.LevelBasic, "%s - Error reading nodes %d, trying again: %v", conn.Name, offset, err)
			infos, err = readNodeAttributes(ctx, cli, batch, maxPerRead)
		}
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "%s - Error reading nodes %d", conn.Name, offset)
			jslog.Log(jslog.LevelNoLog, "%v", err)
			// deviation D27: skipping the batch would leave its points
			// without tags until the next restart, so the discovery fails
			// and the connection is rebuilt.
			return fmt.Errorf("reading node attributes at offset %d: %w", offset, err)
		}

		values, valuesErr := readNodeValues(ctx, cli, batch, maxPerRead, conn.ValueReadChunk())
		valuesOK := valuesErr == nil
		if !valuesOK {
			jslog.Log(jslog.LevelDetailed, "%s - Error reading values %d - %v", conn.Name, offset, valuesErr)
			if sessionLost(cli, valuesErr) {
				// Distinct from an attribute-read failure: the caller
				// shrinks the value-read size before trying again.
				return fmt.Errorf("%w: at offset %d reading %d values at a time: %v",
					errValueReadTooLarge, offset, conn.ValueReadChunk(), valuesErr)
			}
		}

		jslog.Log(jslog.LevelNoLog, "%s -  Autotag - Read %d nodes at offset %d from a total of %d",
			conn.Name, len(infos), offset, len(toRead))

		for i, info := range infos {
			if conn.HasInsertedAddress(info.Address) {
				continue
			}
			if !info.OK {
				continue
			}
			entry, found := browsed.Refs[batch[i]]
			if !found {
				continue
			}

			path, parentName := splitBrowsePath(entry.Path)

			if info.NodeClass == ua.NodeClassMethod && conn.CommandsEnabled {
				if info.Executable && info.UserExecutable {
					jslog.Log(jslog.LevelDetailed, "%s - NodeId %s %v %s Path: %s",
						conn.Name, info.Address, info.NodeClass, info.BrowseName, entry.Path)

					conn.SetNodeDetails(info.Address, &NodeDetails{
						BrowseName:  info.BrowseName,
						DisplayName: info.DisplayName,
						ParentName:  parentName,
						Path:        path,
					})
					enqueueValue(OPCValue{
						CreateCommandForMethod: true,
						AccessLevels:           byte(ua.AccessLevelTypeCurrentWrite),
						SelfPublish:            true,
						Address:                info.Address,
						Asdu:                   "method",
						ServerTimestamp:        time.Now(),
						Quality:                false,
						Cot:                    0,
						ConnNumber:             conn.ProtocolConnectionNumber,
						ConnName:               conn.Name,
						CommonAddress:          "",
						DisplayName:            info.DisplayName,
						ParentName:             parentName,
						Path:                   path,
					})
				}
				// A method is never monitored.
				continue
			}

			// parity: a batch whose values could not all be read is skipped
			// entirely for the value work, methods excepted.
			if !valuesOK || len(values) != len(infos) {
				continue
			}
			dv := values[i]
			if dv == nil {
				continue
			}
			// parity: a node whose value currently reads Bad still gets a
			// tag and is still monitored — its quality is simply false. The
			// C# driver gates on the per-node service result rather than on
			// the value's own status code, which is what lets nodes such as
			// an ExtensionObject reading BadNotReadable through. Skipping
			// them here would mean they never get a tag at all, even once
			// the server starts serving them.

			// parity: the second topic filter, a plain substring test on
			// the whole path rather than a segment match.
			addToMonitoring := len(conn.Topics) == 0
			for _, topic := range conn.Topics {
				if strings.Contains(entry.Path, topic) {
					addToMonitoring = true
					break
				}
			}
			if !addToMonitoring {
				continue
			}

			jslog.Log(jslog.LevelDetailed, "%s - NodeId %s %v %s Path: %s",
				conn.Name, info.Address, info.NodeClass, info.BrowseName, entry.Path)

			conn.SetNodeDetails(info.Address, &NodeDetails{
				BrowseName:  info.BrowseName,
				DisplayName: info.DisplayName,
				ParentName:  parentName,
				Path:        path,
			})

			// parity: the item is registered for monitoring even when the
			// value came back null, so the point starts updating as soon as
			// the server has something to report.
			if !monitored[info.Address] {
				monitored[info.Address] = true
				conn.ListMon = append(conn.ListMon, &monItem{
					NodeID:      info.Address,
					DisplayName: info.BrowseName,
					SamplingMs:  conn.AutoCreateTagSamplingInterval * 1000,
					QueueSize:   uint32(conn.AutoCreateTagQueueSize),
				})
			}

			if dv.Value == nil || dv.Value.Value() == nil {
				continue
			}

			tp, dblValue, strValue, jsonValue, isArray := convertOPCValue(dv)

			// CurrentWrite is a flag inside the access-level bitmask; test
			// the bit rather than comparing for equality, since servers
			// commonly OR in HistoryRead, StatusWrite and friends.
			createCommandForSupervised := conn.CommandsEnabled &&
				info.AccessLevel&byte(ua.AccessLevelTypeCurrentWrite) != 0

			hasSrc := !dv.SourceTimestamp.IsZero()
			srcTime := dv.SourceTimestamp
			if hasSrc {
				srcTime = srcTime.Add(time.Duration(conn.HoursShift * float64(time.Hour)))
			}

			m := meta[info.Address]
			enqueueValue(OPCValue{
				Unit:                       m.unit,
				HasRange:                   m.rangeRank > 0,
				RangeLow:                   m.low,
				RangeHigh:                  m.high,
				CreateCommandForSupervised: createCommandForSupervised,
				AccessLevels:               info.AccessLevel,
				ValueJSON:                  jsonValue,
				SelfPublish:                true,
				Address:                    info.Address,
				IsArray:                    isArray,
				Asdu:                       tp,
				Value:                      dblValue,
				ValueString:                strValue,
				HasSourceTimestamp:         hasSrc,
				SourceTimestamp:            srcTime,
				ServerTimestamp:            time.Now(),
				Quality:                    statusIsGood(dv.Status),
				Cot:                        20,
				ConnNumber:                 conn.ProtocolConnectionNumber,
				ConnName:                   conn.Name,
				CommonAddress:              "",
				DisplayName:                info.DisplayName,
				ParentName:                 parentName,
				Path:                       path,
			})
		}
	}

	jslog.Log(jslog.LevelNoLog, "%s - %d variables added to monitoring.", conn.Name, len(conn.ListMon))
	return nil
}

// metadataProps are the properties of an analog variable (AnalogItemType,
// OPC 10000-8 §5.3.2) that become its tag's unit and limits instead of tags
// of their own. The value is the rank of a range: EURange, the normal
// operating range, comes before InstrumentRange, the range the instrument
// can measure; NumberRange is not standard but some servers use it.
var metadataProps = map[string]int{
	"EngineeringUnits": 0,
	"EURange":          3,
	"InstrumentRange":  2,
	"NumberRange":      1,
}

// isMetadataProperty tells a unit or range property of a variable.
func isMetadataProperty(e refEntry) bool {
	ref := e.Ref
	if ref == nil || ref.BrowseName == nil || ref.NodeClass != ua.NodeClassVariable ||
		ref.ReferenceTypeID == nil || ref.ReferenceTypeID.Namespace() != 0 ||
		ref.ReferenceTypeID.IntID() != id.HasProperty || e.Parent == "" {
		return false
	}
	_, ok := metadataProps[ref.BrowseName.Name]
	return ok
}

// engMeta is what a variable's properties say of it: its unit and its
// range (rangeRank 0 when none was read).
type engMeta struct {
	unit      string
	low, high float64
	rangeRank int
}

// readEngMetadata reads the unit and range properties found by browsing and
// returns them by the key of the variable they describe.
func readEngMetadata(ctx context.Context, cli *opcua.Client, conn *OPCUAConnection, browsed *browseResult, keys []string, maxPerRead uint32) (map[string]engMeta, error) {
	out := map[string]engMeta{}
	if len(keys) == 0 {
		return out, nil
	}
	values, err := readNodeValues(ctx, cli, keys, maxPerRead, conn.ValueReadChunk())
	if err != nil {
		return out, err
	}
	for i, key := range keys {
		e := browsed.Refs[key]
		dv := values[i]
		if dv == nil || !statusIsGood(dv.Status) || dv.Value == nil {
			continue
		}
		m := out[e.Parent]
		v := dv.Value.Value()
		if eo, ok := v.(*ua.ExtensionObject); ok && eo != nil {
			v = eo.Value
		}
		switch x := v.(type) {
		case *ua.EUInformation:
			if u := euUnit(x); u != "" {
				m.unit = u
			}
		case *ua.Range:
			rank := metadataProps[e.Ref.BrowseName.Name]
			if x != nil && validRange(x.Low, x.High) && rank > m.rangeRank {
				m.low, m.high, m.rangeRank = x.Low, x.High, rank
			}
		}
		out[e.Parent] = m
	}
	return out, nil
}

// euUnit is the unit text of an EUInformation: its display name ("°C"),
// else its description.
func euUnit(eu *ua.EUInformation) string {
	if eu == nil {
		return ""
	}
	if eu.DisplayName != nil {
		if t := strings.TrimSpace(eu.DisplayName.Text); t != "" {
			return t
		}
	}
	if eu.Description != nil {
		return strings.TrimSpace(eu.Description.Text)
	}
	return ""
}

func validRange(low, high float64) bool {
	return !math.IsNaN(low) && !math.IsNaN(high) && !math.IsInf(low, 0) && !math.IsInf(high, 0) && low < high
}

// splitBrowsePath turns the full browse path of a node into the path and
// parent name stored on its tag.
//
// deviation D16: unlike the C# original this does not collapse '//', so a
// browse name holding a namespace URI survives intact.
//
// parity: otherwise this reproduces Path.GetDirectoryName + Path.GetFileName plus a
// single regex replacement of "/Objects/", including its edge case — a node
// directly under Objects has no "/Objects/" left to strip, so its path stays
// "/Objects" and its parent is "Objects".
//
//	"/Objects/Boiler/Sensor" -> path "Boiler",   parent "Boiler"
//	"/Objects/Boiler"        -> path "/Objects", parent "Objects"
func splitBrowsePath(full string) (path, parentName string) {
	i := strings.LastIndex(full, "/")
	if i < 0 {
		return "", ""
	}
	dir := full[:i]
	parentName = dir[strings.LastIndex(dir, "/")+1:]
	path = strings.Replace(dir, "/Objects/", "", 1)
	return path, parentName
}

// readNodeAttributes reads the attribute set of a batch of nodes.
func readNodeAttributes(ctx context.Context, cli *opcua.Client, addrs []string, maxPerRead uint32) ([]nodeInfo, error) {
	infos := make([]nodeInfo, len(addrs))
	reads := make([]*ua.ReadValueID, 0, len(addrs)*len(nodeAttrs))

	for i, addr := range addrs {
		nid, err := ua.ParseNodeID(addr)
		if err != nil {
			nid = ua.NewTwoByteNodeID(0)
		}
		infos[i] = nodeInfo{NodeID: nid, Address: addr}
		for _, a := range nodeAttrs {
			reads = append(reads, &ua.ReadValueID{NodeID: nid, AttributeID: a})
		}
	}

	results, err := readChunked(ctx, cli, reads, maxPerRead)
	if err != nil {
		return nil, err
	}
	if len(results) != len(reads) {
		return nil, errShortRead
	}

	for i := range infos {
		base := i * len(nodeAttrs)
		for j, a := range nodeAttrs {
			dv := results[base+j]
			if dv == nil || !statusIsGood(dv.Status) || dv.Value == nil {
				continue
			}
			switch a {
			case ua.AttributeIDNodeClass:
				if n, ok := numericOf(dv.Value.Value()); ok {
					infos[i].NodeClass = ua.NodeClass(int(n))
					infos[i].OK = true
				}
			case ua.AttributeIDBrowseName:
				if qn, ok := dv.Value.Value().(*ua.QualifiedName); ok && qn != nil {
					infos[i].BrowseName = qn.Name
				}
			case ua.AttributeIDDisplayName:
				if lt, ok := dv.Value.Value().(*ua.LocalizedText); ok && lt != nil {
					infos[i].DisplayName = lt.Text
				}
			case ua.AttributeIDUserAccessLevel:
				if n, ok := numericOf(dv.Value.Value()); ok {
					infos[i].AccessLevel = byte(int(n))
				}
			case ua.AttributeIDExecutable:
				if b, ok := dv.Value.Value().(bool); ok {
					infos[i].Executable = b
				}
			case ua.AttributeIDUserExecutable:
				if b, ok := dv.Value.Value().(bool); ok {
					infos[i].UserExecutable = b
				}
			}
		}
	}
	return infos, nil
}

// readNodeValues reads the Value attribute of a batch of nodes.
func readNodeValues(ctx context.Context, cli *opcua.Client, addrs []string, maxPerRead uint32, chunk int) ([]*ua.DataValue, error) {
	reads := make([]*ua.ReadValueID, 0, len(addrs))
	for _, addr := range addrs {
		nid, err := ua.ParseNodeID(addr)
		if err != nil {
			nid = ua.NewTwoByteNodeID(0)
		}
		reads = append(reads, &ua.ReadValueID{NodeID: nid, AttributeID: ua.AttributeIDValue})
	}
	// Values can be arbitrarily large, unlike the fixed-size attributes, so
	// they are read in their own, adaptive, chunk size.
	step := maxPerRead
	if chunk > 0 && (step == 0 || uint32(chunk) < step) {
		step = uint32(chunk)
	}

	results, err := readChunked(ctx, cli, reads, step)
	if err != nil {
		return nil, err
	}
	if len(results) != len(addrs) {
		return nil, errShortRead
	}
	return results, nil
}

// readChunked issues a Read, splitting it so it stays inside the server's
// MaxNodesPerRead. The C# stack does this splitting for the caller.
func readChunked(ctx context.Context, cli *opcua.Client, reads []*ua.ReadValueID, maxPerRead uint32) ([]*ua.DataValue, error) {
	step := len(reads)
	if maxPerRead > 0 && int(maxPerRead) < step {
		step = int(maxPerRead)
	}
	if step < 1 {
		step = 1
	}

	out := make([]*ua.DataValue, 0, len(reads))
	for start := 0; start < len(reads); start += step {
		end := min(start+step, len(reads))

		readCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		resp, err := cli.Read(readCtx, &ua.ReadRequest{
			MaxAge:             0,
			TimestampsToReturn: ua.TimestampsToReturnBoth,
			NodesToRead:        reads[start:end],
		})
		cancel()
		if err != nil {
			return nil, err
		}
		out = append(out, resp.Results...)
	}
	return out, nil
}

var errShortRead = errors.New("server returned fewer results than nodes read")
