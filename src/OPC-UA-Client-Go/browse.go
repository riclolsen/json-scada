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

// Address space browsing. Port of BrowseFullAddressSpaceAsync() in
// AsduReceiveHandler.cs.

package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

const (
	// kMaxSearchDepth and kMaxReferencesPerNode carry the C# names and
	// values so the two implementations stay comparable.
	kMaxSearchDepth       = 128
	kMaxReferencesPerNode = 1000
)

// refEntry is one discovered node: the reference the server returned and
// the browse path the driver built for it.
type refEntry struct {
	Ref  *ua.ReferenceDescription
	Path string
}

// browseResult is the outcome of a full browse: the references by node id
// string, plus the order they were discovered in so the read pass is
// deterministic.
type browseResult struct {
	Refs  map[string]refEntry
	Order []string
}

// browseFullAddressSpace walks the hierarchy below a starting node,
// breadth first, building a browse path for every node it finds.
func browseFullAddressSpace(ctx context.Context, cli *opcua.Client, conn *OPCUAConnection, start *ua.NodeID) (*browseResult, error) {
	began := time.Now()

	out := &browseResult{Refs: map[string]refEntry{}}

	rootPath := "Objects"
	if bn, err := cli.Node(start).BrowseName(ctx); err == nil && bn != nil && bn.Name != "" {
		rootPath = bn.Name
	}

	maxNodesPerBrowse := operationLimit(ctx, cli, id.Server_ServerCapabilities_OperationLimits_MaxNodesPerBrowse)

	pending := []*ua.BrowseDescription{browseDescription(start)}
	duplicates := 0

	for searchDepth := 0; len(pending) > 0 && searchDepth < kMaxSearchDepth; searchDepth++ {
		jslog.Log(jslog.LevelDetailed, "%s - %d: Browse %d nodes after %dms",
			conn.Name, searchDepth+1, len(pending), time.Since(began).Milliseconds())

		var results []*ua.BrowseResult
		var batch []*ua.BrowseDescription
		var unprocessed []*ua.BrowseDescription

		for {
			batch = pending
			if maxNodesPerBrowse > 0 && len(batch) > int(maxNodesPerBrowse) {
				batch = pending[:maxNodesPerBrowse]
			}

			resp, err := cli.Browse(ctx, &ua.BrowseRequest{
				View:                          &ua.ViewDescription{ViewID: ua.NewTwoByteNodeID(0)},
				RequestedMaxReferencesPerNode: kMaxReferencesPerNode,
				NodesToBrowse:                 batch,
			})
			if err != nil {
				// The server could not encode a response this large: halve
				// the batch and try the same level again.
				if isEncodingLimit(err) {
					if maxNodesPerBrowse == 0 {
						maxNodesPerBrowse = uint32(len(batch) / 2)
					} else {
						maxNodesPerBrowse /= 2
					}
					if maxNodesPerBrowse < 1 {
						return nil, err
					}
					jslog.Log(jslog.LevelDetailed, "%s - Browse too large, retrying with %d nodes per browse",
						conn.Name, maxNodesPerBrowse)
					continue
				}
				jslog.Log(jslog.LevelNoLog, "%s - Browse error: %v", conn.Name, err)
				return nil, err
			}

			results = nil
			unprocessed = nil
			for i, br := range resp.Results {
				// The server ran out of continuation points; this node has
				// to be browsed again once others have released theirs.
				if br.StatusCode == ua.StatusBadNoContinuationPoints && i < len(batch) {
					unprocessed = append(unprocessed, batch[i])
				}
				results = append(results, br)
			}
			break
		}

		// Drop the descriptions just browsed so the loop makes progress.
		if len(results) >= len(pending) {
			pending = nil
		} else {
			pending = pending[len(results):]
		}

		// Drain the continuation points of this level.
		//
		// The continued references are merged into the node's own reference
		// list. The C# driver used to collect them into a list it never read,
		// dropping every reference past the first kMaxReferencesPerNode; that
		// was fixed in src/OPC-UA-Client (MergeContinuedReferences), so the
		// two drivers now agree. See D15 in README.md.
		cps := continuationPoints(results)
		for len(cps) > 0 {
			jslog.Log(jslog.LevelDetailed, "%s - BrowseNext %d continuation points.", conn.Name, len(cps))
			nextResp, err := cli.BrowseNext(ctx, &ua.BrowseNextRequest{
				ContinuationPoints:        cps,
				ReleaseContinuationPoints: false,
			})
			if err != nil {
				jslog.Log(jslog.LevelBasic, "%s - BrowseNext error: %v", conn.Name, err)
				break
			}
			mergeContinued(results, nextResp.Results)
			cps = continuationPoints(nextResp.Results)
		}

		// Build the next level, and the browse path of everything found.
		var nextNodes []*ua.NodeID
		for i := range batch {
			if i >= len(results) || results[i].References == nil {
				continue
			}

			parentPath := "/" + rootPath
			if parent, seen := out.Refs[batch[i].NodeID.String()]; seen {
				parentPath = parent.Path
			}

			for _, ref := range results[i].References {
				if ref.NodeID == nil || ref.BrowseName == nil {
					continue
				}
				key := ref.NodeID.String()
				if _, seen := out.Refs[key]; seen {
					duplicates++
					continue
				}

				out.Refs[key] = refEntry{
					Ref:  ref,
					Path: strings.TrimRight(parentPath, "/") + "/" + ref.BrowseName.Name,
				}
				out.Order = append(out.Order, key)

				// Properties are leaves; everything else is expanded.
				if ref.ReferenceTypeID == nil || ref.ReferenceTypeID.IntID() != id.HasProperty {
					nextNodes = append(nextNodes, ua.NewNodeIDFromExpandedNodeID(ref.NodeID))
				}
			}
		}

		for _, n := range nextNodes {
			pending = append(pending, browseDescription(n))
		}
		pending = append(pending, unprocessed...)
	}

	if duplicates > 0 {
		jslog.Log(jslog.LevelDetailed, "%s - Browse Result %d duplicate nodes were ignored.", conn.Name, duplicates)
	}
	jslog.Log(jslog.LevelNoLog, "%s - BrowseFullAddressSpace found %d references on server in %dms.",
		conn.Name, len(out.Refs), time.Since(began).Milliseconds())

	if jslog.Level() >= jslog.LevelDebug {
		for _, key := range out.Order {
			e := out.Refs[key]
			jslog.Log(jslog.LevelDebug, "NodeId %s %v %s Path: %s",
				key, e.Ref.NodeClass, e.Ref.BrowseName.Name, e.Path)
		}
	}

	return out, nil
}

func browseDescription(nid *ua.NodeID) *ua.BrowseDescription {
	return &ua.BrowseDescription{
		NodeID:          nid,
		BrowseDirection: ua.BrowseDirectionForward,
		ReferenceTypeID: ua.NewNumericNodeID(0, id.HierarchicalReferences),
		IncludeSubtypes: true,
		NodeClassMask:   uint32(ua.NodeClassVariable | ua.NodeClassObject | ua.NodeClassMethod),
		ResultMask:      uint32(ua.BrowseResultMaskAll),
	}
}

// continuationPoints collects the points that still have references behind
// them.
func continuationPoints(results []*ua.BrowseResult) [][]byte {
	var cps [][]byte
	for _, r := range results {
		if len(r.ContinuationPoint) > 0 {
			cps = append(cps, r.ContinuationPoint)
		}
	}
	return cps
}

// mergeContinued appends the references of a BrowseNext response back onto
// the results they continue, and carries the new continuation points over.
//
// The responses come back in the order the points were sent, which is the
// order of the results that had one.
func mergeContinued(results []*ua.BrowseResult, continued []*ua.BrowseResult) {
	next := 0
	for _, r := range results {
		if len(r.ContinuationPoint) == 0 {
			continue
		}
		if next >= len(continued) {
			return
		}
		r.References = append(r.References, continued[next].References...)
		r.ContinuationPoint = continued[next].ContinuationPoint
		next++
	}
}

// isEncodingLimit reports whether an error means the response would have
// been too large to encode.
func isEncodingLimit(err error) bool {
	var code ua.StatusCode
	if !errors.As(err, &code) {
		return false
	}
	return code == ua.StatusBadEncodingLimitsExceeded || code == ua.StatusBadResponseTooLarge
}

// operationLimit reads one of the server's operation limits, returning 0
// ("no limit") when the server does not publish it — the same meaning the
// value has in the C# stack.
func operationLimit(ctx context.Context, cli *opcua.Client, limitID uint32) uint32 {
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	v, err := cli.Node(ua.NewNumericNodeID(0, limitID)).Value(readCtx)
	if err != nil || v == nil {
		return 0
	}
	if n, ok := numericOf(v.Value()); ok && n > 0 {
		return uint32(n)
	}
	return 0
}
