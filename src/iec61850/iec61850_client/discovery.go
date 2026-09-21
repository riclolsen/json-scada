/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
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

// Server model discovery: logical devices, logical nodes, data objects,
// data sets and report control blocks. Port of the discovery part of
// Process(Iec61850Connection) in the C# driver, using the ACSI directory
// services of go-iec61850 v0.2.x instead of hand-built MMS name lists.

package main

import (
	"context"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

// discoverServer walks the server directory, links data sets to configured
// entries and activates the report control blocks.
func discoverServer(ctx context.Context, conn *Iec61850Connection) error {
	cli := conn.Client()

	// Discovery is many requests over a large model, so it gets a budget of
	// its own rather than the per-request one.
	budget := conn.requestTimeout() * 20
	if budget < time.Minute {
		budget = time.Minute
	}
	dirCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	lds, err := cli.LogicalDevices(dirCtx)
	if err != nil {
		return err
	}

	for _, ld := range lds {
		jslog.Log(jslog.LevelBasic, "%s LD: %s", conn.Name, ld)

		lns, err := cli.LogicalNodes(dirCtx, ld)
		if err != nil {
			return err
		}

		// One name-list pass covers every class we care about.
		classes := []client.ACSIClass{client.ACSIDataObject, client.ACSIDataSet}
		if conn.UseUrcb {
			classes = append(classes, client.ACSIURCB)
		}
		if conn.UseBrcb {
			classes = append(classes, client.ACSIBRCB)
		}
		found, err := cli.Browse(dirCtx, ld, classes...)
		if err != nil {
			return err
		}

		byLN := map[string]map[client.ACSIClass][]model.ObjectReference{}
		for _, e := range found {
			ln := e.Ref.LN()
			if byLN[ln] == nil {
				byLN[ln] = map[client.ACSIClass][]model.ObjectReference{}
			}
			byLN[ln][e.Class] = append(byLN[ln][e.Class], e.Ref)
		}

		// autoCreateTags: every value-bearing object the server exposes
		// becomes a tag, not only the ones a report happens to carry.
		if conn.AutoCreateTags {
			registerBrowsedPoints(dirCtx, conn, ld)
		}

		for _, ln := range lns {
			jslog.Log(jslog.LevelBasic, "%s  LN: %s", conn.Name, ln)
			lnRef := ld + "/" + ln
			perClass := byLN[ln]

			if conn.Browse {
				for _, doRef := range perClass[client.ACSIDataObject] {
					browseDataObject(dirCtx, conn, doRef)
				}
			}

			for _, dsRef := range perClass[client.ACSIDataSet] {
				discoverDataSet(dirCtx, conn, ld, ln, dsRef)
			}

			// Buffered blocks are offered first: one report stream per data
			// set is enough (§D11), and a buffered one survives a
			// disconnection, which is what the EntryID resync is for.
			for _, rcbRef := range perClass[client.ACSIBRCB] {
				enableRCB(dirCtx, conn, rcbRef, true)
			}
			for _, rcbRef := range perClass[client.ACSIURCB] {
				enableRCB(dirCtx, conn, rcbRef, false)
			}
			_ = lnRef
		}
	}
	return nil
}

// browsedFCs are the functional constraints a discovered data object is
// registered under. They are the ones that carry process values: status
// and measurands. Settings, configuration and description attributes are
// not points, and control blocks are not data.
var browsedFCs = map[model.FC]bool{model.ST: true, model.MX: true}

// registerBrowsedPoints registers every data object of a logical device
// that carries a value, so autoCreateTags covers the whole server model
// rather than only the members of the reports that happen to be active.
//
// One name list per logical device is enough: an IED reports its variables
// as flat MMS item IDs ("GGIO1$ST$Ind1$stVal"), which carry the logical
// node, the functional constraint and the data object. The entry is
// registered at data-object level under its constraint — the same unit a
// report entry names, so a point discovered here and later delivered by a
// report is one entry and one tag, and the same value extraction applies
// to both.
//
// Entries registered here are polled until a report covers them.
func registerBrowsedPoints(ctx context.Context, conn *Iec61850Connection, ld string) {
	names, err := conn.Client().MMS().GetNameList(ctx, mms.ClassNamedVariable, ld)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s Cannot browse %s for tag creation: %v", conn.Name, ld, err)
		return
	}

	added := 0
	controls := map[string]*controlObject{} // data object path -> what was seen

	for _, n := range names {
		parts := strings.Split(n, "$")
		// "LN$FC$DO[$DA...]": anything shorter is the bare logical node or
		// its functional constraint, neither of which is a point.
		if len(parts) < 3 || parts[0] == "" || parts[2] == "" {
			continue
		}
		fc, err := model.ParseFC(parts[1])
		if err != nil {
			continue
		}

		if fc == model.CO {
			noteControlAttribute(controls, ld, parts)
			continue
		}
		if !browsedFCs[fc] {
			continue
		}

		ref := ld + "/" + parts[0] + "." + parts[2]
		key := entryKey(ref, fc)
		entry := conn.Entry(key)
		if entry == nil {
			entry = conn.AddEntry(key, &Iec61850Entry{
				Path:        ref,
				FC:          fc,
				JsTag:       conn.Name + ":" + ref,
				AutoPublish: true,
			})
			if entry.AutoPublish {
				added++
			}
		}
		// The direct attributes of the object, in the order the server
		// lists them: that is what lets the value be read by name (stVal,
		// mag, …) rather than by guessing from the types.
		if len(parts) > 3 {
			conn.AddEntryChildOnce(entry, parts[3])
		}
	}
	if added > 0 {
		jslog.Log(jslog.LevelBasic, "%s %s: %d browsed object(s) registered for tag creation", conn.Name, ld, added)
	}

	if conn.CommandsEnabled {
		registerControlObjects(ctx, conn, ld, controls)
	}
}

// controlObject is what the name list says about a controllable object.
type controlObject struct {
	ref      string // "LD/LN.DO[.SDO]"
	item     string // "LN$CO$DO[$SDO]"
	hasOper  bool
	hasSBO   bool // select-before-operate, either security level
	analogue bool // the control value is an AnalogueValue ("ctlVal$f"/"$i")
}

// noteControlAttribute records what an "LN$CO$..." item reveals about the
// controllable object it belongs to.
func noteControlAttribute(into map[string]*controlObject, ld string, parts []string) {
	// The control attribute is the last component that names one:
	// "LN$CO$DO[$SDO]$Oper[$ctlVal[$f]]".
	phase := -1
	for i := 3; i < len(parts); i++ {
		switch parts[i] {
		case "Oper", "SBO", "SBOw", "Cancel":
			phase = i
		}
	}
	if phase < 0 {
		return
	}
	doPath := parts[2:phase]
	if len(doPath) == 0 {
		return
	}
	item := parts[0] + "$CO$" + strings.Join(doPath, "$")
	co := into[item]
	if co == nil {
		co = &controlObject{ref: ld + "/" + parts[0] + "." + strings.Join(doPath, "."), item: item}
		into[item] = co
	}
	switch parts[phase] {
	case "Oper":
		co.hasOper = true
	case "SBO", "SBOw":
		co.hasSBO = true
	}
	// "…$Oper$ctlVal$f" or "$i": the control value is an analogue one.
	if phase+2 < len(parts) && parts[phase+1] == "ctlVal" {
		co.analogue = true
	}
}

// registerControlObjects registers the controllable objects of a logical
// device and queues a command tag for each, linked to the supervised point
// of the same data object.
func registerControlObjects(ctx context.Context, conn *Iec61850Connection, ld string, controls map[string]*controlObject) {
	queued := 0
	for _, co := range controls {
		if !co.hasOper {
			continue // status-only, nothing to command
		}
		key := entryKey(co.ref, model.CO)
		if conn.Entry(key) != nil {
			continue // already configured or registered
		}
		// Register the entry so a command can be dispatched in this
		// session, before the tag is reloaded at the next start.
		conn.AddEntry(key, &Iec61850Entry{
			Path:        co.ref,
			FC:          model.CO,
			JsTag:       conn.Name + ":" + co.ref,
			AutoPublish: true,
		})

		isDigital, asdu := controlValueKind(ctx, conn, ld, co)
		enqueueCommandTag(CommandTag{
			ConnNumber: conn.ProtocolConnectionNumber,
			ConnName:   conn.Name,
			Ref:        co.ref,
			IsDigital:  isDigital,
			UseSBO:     co.hasSBO,
			Asdu:       asdu,
		})
		queued++
	}
	if queued > 0 {
		jslog.Log(jslog.LevelBasic, "%s %s: %d controllable object(s) registered for command tag creation",
			conn.Name, ld, queued)
	}
}

// controlValueKind resolves the type of a control value: the type of the
// Oper structure's ctlVal decides whether the command tag is digital or
// analogue. The name list alone cannot tell a boolean from an integer, so
// the type description is read; it is one request per controllable object,
// and there are far fewer of those than data objects.
func controlValueKind(ctx context.Context, conn *Iec61850Connection, ld string, co *controlObject) (bool, string) {
	if co.analogue {
		return false, mmsTypeName(mms.TypeStructure)
	}
	spec, err := conn.Client().MMS().GetVariableAccessAttributes(ctx, ld, co.item+"$Oper")
	if err == nil && spec != nil {
		for _, comp := range spec.Components {
			if comp.Name != "ctlVal" || comp.Spec == nil {
				continue
			}
			switch comp.Spec.Kind {
			case mms.TypeBoolean:
				return true, mmsTypeName(mms.TypeBoolean)
			case mms.TypeBitString:
				// A double-point control carries a two-bit position.
				return true, mmsTypeName(mms.TypeBitString)
			default:
				return false, mmsTypeName(comp.Spec.Kind)
			}
		}
	}
	// Unknown: a single-point control is by far the most common.
	jslog.Log(jslog.LevelDetailed, "%s %s: cannot read the control value type, assuming digital: %v", conn.Name, co.ref, err)
	return true, mmsTypeName(mms.TypeBoolean)
}

// browseDataObject logs the attributes of a data object, the equivalent of
// the C# driver's GetDataDirectoryFC walk (only when browse is enabled).
func browseDataObject(ctx context.Context, conn *Iec61850Connection, doRef model.ObjectReference) {
	cli := conn.Client()
	do := doRef.Path()
	if len(do) < 2 {
		return
	}
	jslog.Log(jslog.LevelBasic, "%s    DO: %s", conn.Name, strings.Join(do[1:], "."))

	for _, fc := range allFCs {
		children, err := cli.DataDirectory(ctx, doRef, fc)
		if err != nil || len(children) == 0 {
			continue
		}
		for _, child := range children {
			daRef := doRef.Child(child)
			domain, item := daRef.ToMMS(fc)
			spec, err := cli.MMS().GetVariableAccessAttributes(ctx, domain, item)
			if err != nil {
				continue
			}
			jslog.Log(jslog.LevelBasic, "%s      DA/SDO: [%s] %s : %s(%d) ... %s",
				conn.Name, fc, child, spec.Kind, specSize(spec), daRef)
			if spec.Kind == mms.TypeStructure {
				for _, comp := range spec.Components {
					jslog.Log(jslog.LevelBasic, "%s           %s : %s ... %s.%s",
						conn.Name, comp.Name, comp.Spec.Kind, daRef, comp.Name)
				}
			}
		}
	}
}

// allFCs are the functional constraints a data object can expose values
// under, used for the diagnostic browse.
var allFCs = []model.FC{
	model.ST, model.MX, model.SP, model.SV, model.CF, model.DC,
	model.SG, model.SE, model.OR, model.BL, model.EX, model.CO,
}

func specSize(spec *mms.TypeSpec) int {
	if spec == nil {
		return 0
	}
	if spec.Kind == mms.TypeStructure {
		return len(spec.Components)
	}
	if spec.Kind == mms.TypeArray {
		return spec.Elements
	}
	return spec.Size
}

// discoverDataSet lists a data set's members and links the ones that match
// a configured entry, collecting the names of their child attributes.
func discoverDataSet(ctx context.Context, conn *Iec61850Connection, ld, ln string, dsRef model.ObjectReference) {
	cli := conn.Client()
	path := dsRef.Path()
	if len(path) < 2 {
		return
	}
	dsName := path[1]
	dsFullName := ld + "/" + ln + "." + dsName
	jslog.Log(jslog.LevelBasic, "%s    Dataset: %s", conn.Name, dsFullName)
	conn.Datasets = append(conn.Datasets, dsName)

	members, err := cli.MMS().GetNamedVariableListAttributes(ctx, ld, ln+"$"+dsName)
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "%s     %s - cannot read members: %v", conn.Name, dsFullName, err)
		return
	}

	for _, m := range members {
		jslog.Log(jslog.LevelBasic, "%s     %s -> %s", conn.Name, dsFullName, m.Item)
		ref, fc := model.FromMMS(m.Domain, m.Item)
		entry := conn.Entry(entryKey(string(ref), fc))
		if entry == nil {
			continue
		}
		conn.SetEntryDataSet(entry, dsFullName)
		jslog.Log(jslog.LevelBasic, "%s       Found desired entry %s", conn.Name, entry.Path)
		if conn.EntryNeedsChilds(entry) {
			spec, err := cli.MMS().GetVariableAccessAttributes(ctx, m.Domain, m.Item)
			if err == nil && spec != nil {
				for _, comp := range spec.Components {
					jslog.Log(jslog.LevelBasic, "%s         Child %s", conn.Name, comp.Name)
					conn.AddEntryChild(entry, comp.Name)
				}
			}
		}
	}
}
