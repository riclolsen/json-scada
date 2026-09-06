/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
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

// Dynamic data model builder, port of ModelBuilder.cs. One logical device
// per topic (group1), points exposed as GGIO data objects, per-LD data sets
// and buffered/unbuffered report control blocks.

package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

// Model bounds. These exist to keep every MMS response small enough for
// clients that negotiate a small PDU; see the README for the measurements
// behind them. They are the C# driver's values, so both drivers present the
// same model shape.
const (
	// Max data objects per GGIO instance, counting all categories together.
	MaxDataObjectsPerLN = 30
	// Descriptions are published in 'd' (FC=DC) and one DC read returns
	// every description of the logical node, so they are truncated here.
	MaxDescriptionLength = 32
	// Max FCDA entries per data set: reading one returns every member at
	// once, and an integrity/GI report of the full set must fit a PDU.
	EntriesPerDataSet = 40
	// Report control blocks all live in the LD's LLN0, and a browse of
	// LLN0[BR]/[RP] returns every block of that family in one response.
	MaxRcbPerLLN0 = 7
	// Upper bound on RCB instances per data set (one per client reporting
	// on the same data set concurrently).
	MaxRcbCopiesPerDataSet = 4
)

// doCost is charged against MaxDataObjectsPerLN. A string point carries a
// VisibleString value that may be far larger than a status or measurand, so
// it consumes more of the logical node's budget.
func doCost(kind PointKind) int {
	if kind == KindVSS {
		return 8
	}
	return 1
}

// MappedPoint associates a json-scada tag with its place in the model.
type MappedPoint struct {
	Tag       string
	Type      string // realtimeData type, lowercased: digital, analog, string, json
	Kind      PointKind
	IsCommand bool
	PointKey  float64
	ObjRef    model.ObjectReference // "IEDNameLD/GGIO1.Ind1"
	FC        model.FC              // ST or MX, the family of value/q/t

	ValueRef model.ObjectReference
	QRef     model.ObjectReference
	TRef     model.ObjectReference

	// Conversion factors applied to a routed command value.
	Kconv1 float64
	Kconv2 float64

	// Command routing, copied from the tag with its original BSON types.
	SrcConnectionNumber float64
	SrcCommonAddress    any
	SrcObjectAddress    any
	SrcASDU             any
	SrcCommandDuration  float64
	SrcCommandUseSBO    bool
}

// BuiltModel is everything the rest of the driver needs from a build.
type BuiltModel struct {
	Model        *model.Model
	ByTag        map[string]*MappedPoint
	ByCtlObjRef  map[string]*MappedPoint
	Order        []*MappedPoint // build order, for the manifest
	LogicalNodes int
	Devices      int
}

// modelBounds are derived from maxClientConnections at build time.
type modelBounds struct {
	rcbCopies      int
	maxPointsPerLD int
}

func computeModelBounds(conn *ServerConnection) modelBounds {
	maxClients := int(conn.MaxClientConnections)
	if maxClients < 1 {
		maxClients = 1
	}
	rcbCopies := maxClients
	if rcbCopies > MaxRcbCopiesPerDataSet {
		rcbCopies = MaxRcbCopiesPerDataSet
		jslog.Log(jslog.LevelBasic, "maxClientConnections=%d exceeds the %d report control block instances created "+
			"per data set; clients beyond that cannot enable reports on the same data set concurrently.",
			maxClients, MaxRcbCopiesPerDataSet)
	}
	dataSetBudget := MaxRcbPerLLN0 / rcbCopies
	if dataSetBudget < 1 {
		dataSetBudget = 1
	}
	span := dataSetBudget - 1
	if span < 1 {
		span = 1
	}
	b := modelBounds{rcbCopies: rcbCopies, maxPointsPerLD: EntriesPerDataSet * span}
	jslog.Log(jslog.LevelBasic, "Model bounds: %d RCB copies/data set, <= %d points per logical device, "+
		"<= %d entries per data set, <= %d objects per logical node.",
		b.rcbCopies, b.maxPointsPerLD, EntriesPerDataSet, MaxDataObjectsPerLN)
	return b
}

// ldContext accumulates the state needed to lay out points inside one
// logical device.
type ldContext struct {
	ldInst string
	ldName string
	ld     *model.LogicalDevice
	lln0   *model.LogicalNode

	ggios    map[int]*model.LogicalNode
	ggioIdx  int
	ggioCost int
	counters map[string]int

	statusEntries []model.FCDA
	mxEntries     []model.FCDA
	memberRefs    []string
}

// BuildModel lays out the whole model and returns it with the tag maps.
func BuildModel(points []*Point, conn *ServerConnection) *BuiltModel {
	iedName := sanitizeMms(conn.Name, 20)
	if iedName == "" {
		iedName = "JSONSCADA"
	}
	jslog.Log(jslog.LevelBasic, "IED name: %s", iedName)

	bounds := computeModelBounds(conn)

	built := &BuiltModel{
		Model:       &model.Model{Name: iedName},
		ByTag:       map[string]*MappedPoint{},
		ByCtlObjRef: map[string]*MappedPoint{},
	}

	// Group by group1 (empty -> GEN), keeping the _id order of the points.
	var groups []string
	byGroup := map[string][]*Point{}
	for _, p := range points {
		g1 := p.Group1
		if g1 == "" {
			g1 = "GEN"
		}
		if _, seen := byGroup[g1]; !seen {
			groups = append(groups, g1)
		}
		byGroup[g1] = append(byGroup[g1], p)
	}

	// Split oversized topics across several logical devices so every LLN0
	// stays small.
	type batch struct {
		name   string
		points []*Point
	}
	var batches []batch
	for _, g1 := range groups {
		pts := byGroup[g1]
		if len(pts) <= bounds.maxPointsPerLD {
			batches = append(batches, batch{g1, pts})
			continue
		}
		part := 0
		for _, chunk := range chunkPoints(pts, bounds.maxPointsPerLD) {
			part++
			name := g1
			if part > 1 {
				name = fmt.Sprintf("%s_%d", g1, part)
			}
			batches = append(batches, batch{name, chunk})
		}
		jslog.Log(jslog.LevelBasic, "Topic '%s' has %d points - split across %d logical devices.", g1, len(pts), part)
	}

	ldNameBudget := 62 - len(iedName)
	if ldNameBudget < 8 {
		ldNameBudget = 8
	}
	usedLdInst := map[string]bool{}
	descCount, proxyCount := 0, 0

	for _, b := range batches {
		ldInst := uniqueName(sanitizeMms(b.name, ldNameBudget), usedLdInst)
		ctx := &ldContext{
			ldInst:   ldInst,
			ldName:   iedName + ldInst,
			ggios:    map[int]*model.LogicalNode{},
			counters: map[string]int{},
			// Start "full" so the first point opens GGIO1.
			ggioCost: MaxDataObjectsPerLN,
		}
		ctx.ld = &model.LogicalDevice{Name: ctx.ldName, Inst: ldInst}
		ctx.lln0 = &model.LogicalNode{Name: "LLN0", Class: "LLN0", Objects: []*model.DataObject{
			model.NewDataObject("Beh", model.CDCENS),
			model.NewDataObject("Health", model.CDCENS),
			model.NewDataObject("NamPlt", model.CDCLPL),
			newLastApplError(),
		}}
		ctx.ld.Nodes = append(ctx.ld.Nodes, ctx.lln0)

		// LPHD with the Proxy flag: the IEC 61850-90-2 gateway marker.
		proxy := model.NewDataObject("Proxy", model.CDCSPS)
		if setAttrValue(proxy, "stVal", mms.NewBool(true)) {
			proxyCount++
		}
		lphd := &model.LogicalNode{Name: "LPHD1", Class: "LPHD", Objects: []*model.DataObject{
			model.NewDataObject("PhyNam", model.CDCDPL),
			model.NewDataObject("PhyHealth", model.CDCINS),
			proxy,
		}}
		ctx.ld.Nodes = append(ctx.ld.Nodes, lphd)

		for _, p := range b.points {
			if mp := mapPoint(ctx, p, &descCount); mp != nil {
				built.ByTag[mp.Tag] = mp
				built.Order = append(built.Order, mp)
				if mp.IsCommand {
					built.ByCtlObjRef[string(mp.ObjRef)] = mp
				}
			}
		}

		buildDataSetsAndReports(ctx, bounds, int(conn.MaxQueueSize))

		// GGIO instances follow LLN0 and LPHD1 in the node list.
		for i := 1; i <= ctx.ggioIdx; i++ {
			if ln, ok := ctx.ggios[i]; ok {
				ctx.ld.Nodes = append(ctx.ld.Nodes, ln)
			}
		}
		built.LogicalNodes += len(ctx.ld.Nodes)
		built.Model.Devices = append(built.Model.Devices, ctx.ld)
	}

	built.Devices = len(built.Model.Devices)
	jslog.Log(jslog.LevelBasic, "Applied model properties (proxy flags: %d, descriptions: %d).", proxyCount, descCount)
	jslog.Log(jslog.LevelBasic, "Model built: %d logical device(s), %d point(s), %d command(s).",
		built.Devices, len(built.ByTag), len(built.ByCtlObjRef))
	return built
}

// mapPoint places one point in the current logical device.
func mapPoint(ctx *ldContext, p *Point, descCount *int) *MappedPoint {
	isCommand := p.IsCommand()
	typ := strings.ToLower(p.Type)

	var kind PointKind
	var prefix string
	switch {
	case isCommand && typ == "analog":
		kind, prefix = KindAPC, "AnOut"
	case isCommand:
		kind, prefix = KindSPC, "SPCSO"
	case typ == "analog":
		kind, prefix = KindMV, "AnIn"
	case typ == "string" || typ == "json":
		kind, prefix = KindVSS, "Str"
	default:
		kind, prefix = KindSPS, "Ind"
	}

	// Open a new GGIO instance once the current one is full.
	cost := doCost(kind)
	if ctx.ggioCost+cost > MaxDataObjectsPerLN {
		ctx.ggioIdx++
		ctx.ggioCost = 0
		ctx.counters = map[string]int{}
	}
	ctx.counters[prefix]++
	ctx.ggioCost += cost
	ggio := getOrCreateGGIO(ctx, ctx.ggioIdx)
	doName := fmt.Sprintf("%s%d", prefix, ctx.counters[prefix])

	var do *model.DataObject
	if isCommand {
		do = newCommandObject(doName, kind, p.SrcCommandUseSBO)
	} else {
		do = newMonitorObject(doName, kind)
	}
	ggio.Objects = append(ggio.Objects, do)

	objRef := model.ObjectReference(ctx.ldName + "/" + ggio.Name + "." + doName)
	path, fc := valueAttrPath(kind)
	valueRef := objRef
	for _, seg := range path {
		valueRef = valueRef.Child(seg)
	}

	mp := &MappedPoint{
		Tag:       p.Tag,
		Type:      typ,
		Kind:      kind,
		IsCommand: isCommand,
		PointKey:  p.ID,
		ObjRef:    objRef,
		FC:        fc,
		ValueRef:  valueRef,
		QRef:      objRef.Child("q"),
		TRef:      objRef.Child("t"),

		Kconv1: p.Kconv1,
		Kconv2: p.Kconv2,

		SrcConnectionNumber: p.SrcConnectionNumber,
		SrcCommonAddress:    p.SrcCommonAddress,
		SrcObjectAddress:    p.SrcObjectAddress,
		SrcASDU:             p.SrcASDU,
		SrcCommandDuration:  p.SrcCommandDuration,
		SrcCommandUseSBO:    p.SrcCommandUseSBO,
	}

	// The description attribute makes the model self-documenting; the full
	// text stays in the manifest.
	desc := p.Description
	if desc == "" {
		desc = p.Tag
	}
	if len(desc) > MaxDescriptionLength {
		desc = desc[:MaxDescriptionLength]
	}
	if setAttrValue(do, "d", mms.NewVisibleString(desc)) {
		*descCount++
	}

	// Data set membership, monitoring points only: a DO-level FCDA carries
	// value, quality and timestamp in one entry.
	if !isCommand {
		entry := model.FCDA{Ref: objRef, FC: fc}
		if fc == model.MX {
			ctx.mxEntries = append(ctx.mxEntries, entry)
		} else {
			ctx.statusEntries = append(ctx.statusEntries, entry)
		}
		ctx.memberRefs = append(ctx.memberRefs, string(objRef))
	}
	return mp
}

func getOrCreateGGIO(ctx *ldContext, idx int) *model.LogicalNode {
	if ln, ok := ctx.ggios[idx]; ok {
		return ln
	}
	ln := &model.LogicalNode{
		Name:  fmt.Sprintf("GGIO%d", idx),
		Class: "GGIO",
		Objects: []*model.DataObject{
			model.NewDataObject("Beh", model.CDCENS),
		},
	}
	ctx.ggios[idx] = ln
	return ln
}

// buildDataSetsAndReports fills the LD's LLN0 with the data sets and the
// buffered/unbuffered report control blocks over them.
func buildDataSetsAndReports(ctx *ldContext, bounds modelBounds, maxQueueSize int) {
	confRev := fnv32(strings.Join(ctx.memberRefs, "|"))

	for i, chunk := range chunkFCDA(ctx.statusEntries, EntriesPerDataSet) {
		dsName := fmt.Sprintf("DS_ST_%d", i+1)
		ctx.lln0.DataSets = append(ctx.lln0.DataSets, &model.DataSet{Name: dsName, Entries: chunk})
		addReportControls(ctx, dsName, "ST", i+1, confRev, bounds.rcbCopies, maxQueueSize)
	}
	for i, chunk := range chunkFCDA(ctx.mxEntries, EntriesPerDataSet) {
		dsName := fmt.Sprintf("DS_MX_%d", i+1)
		ctx.lln0.DataSets = append(ctx.lln0.DataSets, &model.DataSet{Name: dsName, Entries: chunk})
		addReportControls(ctx, dsName, "MX", i+1, confRev, bounds.rcbCopies, maxQueueSize)
	}
}

// addReportControls creates one buffered and one unbuffered control block
// over a data set. The server materialises RptEnabled instances of each,
// named brcbST0101, brcbST0102, ... exactly as the C# driver named them.
func addReportControls(ctx *ldContext, dsName, cat string, dsIdx int, confRev uint32, copies, maxQueueSize int) {
	trg := model.TrgDataChange | model.TrgQualityChange | model.TrgIntegrity | model.TrgGI
	opt := model.OptSeqNum | model.OptTimeOfEntry | model.OptReasonCode |
		model.OptDataSetName | model.OptConfRev

	ctx.lln0.ReportControls = append(ctx.lln0.ReportControls,
		&model.ReportControl{
			Name:         fmt.Sprintf("brcb%s%02d", cat, dsIdx),
			DataSet:      dsName,
			ConfRev:      confRev,
			Buffered:     true,
			BufTime:      500,
			TrgOps:       trg,
			OptFlds:      opt | model.OptEntryID | model.OptBufOvfl,
			RptEnabled:   copies,
			MaxQueueSize: maxQueueSize,
		},
		&model.ReportControl{
			Name:       fmt.Sprintf("urcb%s%02d", cat, dsIdx),
			DataSet:    dsName,
			ConfRev:    confRev,
			Buffered:   false,
			BufTime:    500,
			TrgOps:     trg,
			OptFlds:    opt,
			RptEnabled: copies,
		})
}

// ---- helpers ------------------------------------------------------------

// sanitizeMms reduces a string to a valid MMS identifier ([A-Za-z0-9_], not
// starting with a digit), truncating to maxLen with a stable hash suffix so
// names stay collision-resistant.
func sanitizeMms(s string, maxLen int) string {
	var b strings.Builder
	for _, c := range s {
		if c < 128 && (unicode.IsLetter(c) || unicode.IsDigit(c)) {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	r := strings.Trim(b.String(), "_")
	if r == "" {
		r = "P"
	}
	if r[0] >= '0' && r[0] <= '9' {
		r = "P" + r
	}
	if len(r) > maxLen {
		cut := maxLen - 5
		if cut < 1 {
			cut = 1
		}
		r = r[:cut] + fmt.Sprintf("_%04X", fnv32(s)&0xFFFF)
	}
	return r
}

func uniqueName(base string, used map[string]bool) string {
	name := base
	for i := 2; used[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	used[name] = true
	return name
}

func fnv32(s string) uint32 {
	var hash uint32 = 2166136261
	for _, c := range s {
		hash ^= uint32(c)
		hash *= 16777619
	}
	if hash == 0 {
		return 1
	}
	return hash
}

func chunkPoints(list []*Point, size int) [][]*Point {
	var out [][]*Point
	for i := 0; i < len(list); i += size {
		end := i + size
		if end > len(list) {
			end = len(list)
		}
		out = append(out, list[i:end])
	}
	return out
}

func chunkFCDA(list []model.FCDA, size int) [][]model.FCDA {
	var out [][]model.FCDA
	for i := 0; i < len(list); i += size {
		end := i + size
		if end > len(list) {
			end = len(list)
		}
		out = append(out, list[i:end])
	}
	return out
}
