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

// Report control block activation. go-iec61850 v0.2.x supports several
// concurrent subscriptions per association, each with its own callback, so
// this is a thin wrapper: read the RCB, set the trigger options the C#
// driver used, subscribe, and ask for a general interrogation.

package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

// rcbState is what the report callback needs to know about the control
// block that produced a report.
type rcbState struct {
	ref        string // "LD/LN.RP.name"
	rptID      string
	dataSetRef string // as reported by the RCB, MMS notation "LD/LN$Name"
	dataSetDot string // dotted form, "LD/LN.Name"
	buffered   bool
	collision  bool // another RCB advertises the same RptID
}

// rcbToMMS splits an RCB reference into the MMS domain and item id:
// "LD/LLN0.RP.urcb01" -> ("LD", "LLN0$RP$urcb01").
func rcbToMMS(ref model.ObjectReference) (string, string) {
	return ref.LD(), strings.Join(ref.Path(), "$")
}

// dottedDataSet converts an RCB's DatSet value ("LD/LLN0$Events") to the
// dotted form used elsewhere ("LD/LLN0.Events").
func dottedDataSet(ds string) string {
	ld, rest, ok := strings.Cut(ds, "/")
	if !ok {
		return ds
	}
	return ld + "/" + strings.ReplaceAll(rest, "$", ".")
}

// enableRCB configures and activates one report control block.
func enableRCB(ctx context.Context, conn *Iec61850Connection, ref model.ObjectReference, buffered bool) {
	kind := "URCB"
	if buffered {
		kind = "BRCB"
	}
	rpname := string(ref)
	jslog.Log(jslog.LevelBasic, "%s %s: %s", conn.Name, kind, rpname)

	if len(conn.Topics) > 0 && !containsString(conn.Topics, rpname) {
		jslog.Log(jslog.LevelBasic, "%s Report will not be activated! not in topics list.", conn.Name)
		return
	}
	if buffered {
		conn.Brcb = append(conn.Brcb, rpname)
	} else {
		conn.Urcb = append(conn.Urcb, rpname)
	}

	cli := conn.Client()
	rcb, err := cli.GetRCB(ctx, ref)
	if err != nil {
		// parity: the C# log message carries this typo.
		jslog.Log(jslog.LevelBasic, "%s %s: IED GetRCB excepion - %v", conn.Name, kind, err)
		return
	}
	rcb.Buffered = buffered

	domain, item := rcbToMMS(ref)

	// A report is matched to its subscription by RptID. When the IED leaves
	// RptID empty it reports the RCB reference instead, so adopt that here;
	// EnableReporting never writes RptID, this only fixes the matching.
	if rcb.RptID == "" {
		rcb.RptID = domain + "/" + item
		jslog.Log(jslog.LevelDetailed, "%s %s: empty RptID, matching reports on '%s'", conn.Name, kind, rcb.RptID)
	}

	// The dataset members are what gives a report entry its object
	// reference. Without them nothing can be mapped to a tag, so do not
	// enable a report we would not be able to interpret.
	dsList := rcb.DataSet
	if dsList == "" {
		jslog.Log(jslog.LevelBasic, "%s %s: %s has no data set - not activated", conn.Name, kind, rpname)
		return
	}
	if _, list, ok := strings.Cut(dsList, "/"); ok {
		if members, err := cli.MMS().GetNamedVariableListAttributes(ctx, domain, list); err != nil || len(members) == 0 {
			jslog.Log(jslog.LevelBasic, "%s %s: %s dataset members unavailable - report entries cannot be mapped, not activated",
				conn.Name, kind, rpname)
			return
		}
	}

	st := &rcbState{
		ref:        rpname,
		rptID:      rcb.RptID,
		dataSetRef: rcb.DataSet,
		dataSetDot: dottedDataSet(rcb.DataSet),
		buffered:   buffered,
	}
	conn.mu.Lock()
	if conn.RcbByDataSet == nil {
		conn.RcbByDataSet = map[string]*rcbState{}
	}
	// One report stream per data set. A server offers several control block
	// instances over the same data set — indexed blocks, spares for other
	// clients, a buffered and an unbuffered one — and every one of them
	// delivers the same values, so the same tags would be written once per
	// block. One is enough (deviation D11). Buffered blocks are activated
	// first, so a data set that has one is served by it.
	dsKey := st.dataSetRef
	if prev := conn.RcbByDataSet[dsKey]; prev != nil {
		conn.mu.Unlock()
		jslog.Log(jslog.LevelBasic, "%s %s: %s reports the same data set as %s - not activated",
			conn.Name, kind, rpname, prev.ref)
		return
	}
	// Two blocks over different data sets may still advertise one RptID,
	// which the report format cannot tell apart; the callback then falls
	// back to matching on the data set name.
	if prev, dup := conn.RcbByRptID[rcb.RptID]; dup {
		prev.collision = true
		st.collision = true
		jslog.Log(jslog.LevelBasic, "%s %s: RptID '%s' collides with %s - reports will be matched by dataset",
			conn.Name, kind, rcb.RptID, prev.ref)
	}
	conn.RcbByRptID[rcb.RptID] = st
	conn.RcbByDataSet[dsKey] = st
	conn.mu.Unlock()

	configureRCB(conn, rcb, rpname, buffered)

	enable := func() (*client.ReportSubscription, error) {
		return cli.EnableReporting(ctx, rcb, func(rep *client.Report) {
			reportHandler(conn, st, rep)
		})
	}
	sub, err := enable()
	// The saved EntryID is gone from the IED's buffer (it restarted or
	// purged it), so the IED refuses it. Retrying with the same ID would
	// fail forever: resume from the IED's own position instead and forget
	// the ID. Reports buffered in between are lost.
	if err != nil && rcb.ResyncEntryID != nil && entryIDRefused(err) {
		jslog.Log(jslog.LevelBasic, "%s BRCB: %s - entryId %s refused by the IED, enabling without resync",
			conn.Name, rpname, entryIDString(rcb.ResyncEntryID))
		conn.ForgetLastReportID(rpname)
		rcb.ResyncEntryID = nil
		sub, err = enable()
	}
	if err != nil {
		// An RCB left enabled by a previous association refuses
		// configuration writes; disable it and try once more. A block
		// another client still holds refuses the disable too, and then a
		// retry cannot succeed.
		if derr := disableRCB(ctx, cli, domain, item); derr != nil {
			jslog.Log(jslog.LevelBasic, "%s %s: %s is held by another client (RptEna=false refused - %v)",
				conn.Name, kind, rpname, derr)
		} else {
			sub, err = enable()
		}
	}
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s %s: IED SetRCB exception - %v", conn.Name, kind, err)
		conn.mu.Lock()
		// Only drop our own registrations: another control block may hold
		// this RptID. Releasing the data set lets the next instance over it
		// be tried instead.
		if cur, ok := conn.RcbByRptID[rcb.RptID]; ok && cur == st {
			delete(conn.RcbByRptID, rcb.RptID)
		}
		if cur, ok := conn.RcbByDataSet[dsKey]; ok && cur == st {
			delete(conn.RcbByDataSet, dsKey)
		}
		conn.mu.Unlock()
		return
	}
	conn.AddSubscription(sub)

	if err := cli.TriggerGI(ctx, rcb); err != nil {
		jslog.Log(jslog.LevelBasic, "%s %s: IED SetRCB exception - %v", conn.Name, kind, err)
	}
}

// configureRCB sets what the driver asks of a report control block before
// enabling it: triggers, integrity period, optional fields and, for a
// buffered one, the EntryID to resume after.
func configureRCB(conn *Iec61850Connection, rcb *client.RCB, rpname string, buffered bool) {
	// enableRCB requests a GI as soon as the block is enabled, and a server
	// only answers a GI request whose trigger is enabled in TrgOps
	// (IEC 61850-7-2). Without it a conformant IED ignores the request and
	// nothing arrives until the first change or integrity period. The C#
	// driver left GI out.
	rcb.TrgOps = model.TrgDataChange | model.TrgIntegrity | model.TrgGI
	rcb.IntgPd = time.Duration(conn.Class0ScanInterval) * time.Second
	// parity: the IED keeps its own buffer time. GetRCB reads BufTm and
	// EnableReporting writes back any non-zero value, a write the C# driver
	// never made and an IED with fixed report settings may refuse, losing
	// the block for nothing.
	rcb.BufTm = 0
	// The C# driver also requested DATA_REFERENCE. Report entries are
	// identified here from the data set members instead, and the client
	// decoder discards the reference strings, so asking for them buys
	// nothing and costs one string per element per report. It is also a
	// risk: a report is decoded by position, so a server that announces
	// data references without sending them shifts every value after the
	// flags (deviation D10).
	rcb.OptFlds = model.OptSeqNum | model.OptTimeOfEntry | model.OptReasonCode |
		model.OptDataSetName | model.OptConfRev
	if buffered {
		rcb.OptFlds |= model.OptEntryID
		rcb.ResyncEntryID = resyncEntryID(conn, rpname)
		if rcb.ResyncEntryID != nil {
			jslog.Log(jslog.LevelBasic, "%s BRCB: %s - Last seen entryId: %s", conn.Name, rpname, entryIDString(rcb.ResyncEntryID))
		}
	}
}

// resyncEntryID is the EntryID a buffered block resumes after: the last one
// this driver saw on it, or nil when it has seen none. Then no EntryID is
// written and the IED delivers from its own position. The C# driver wrote
// all zeros instead, which some IEDs refuse outright.
func resyncEntryID(conn *Iec61850Connection, rcbRef string) []byte {
	saved, ok := conn.LastReportID(rcbRef)
	if !ok || len(saved) == 0 || isZeroEntryID(saved) {
		return nil
	}
	return saved
}

func isZeroEntryID(id []byte) bool {
	for _, b := range id {
		if b != 0 {
			return false
		}
	}
	return true
}

// entryIDRefused reports whether enabling failed because the IED refused the
// resync EntryID (IEC 61850-7-2: an entry its buffer does not hold). Settings
// can be refused with the same code, so the write is identified by the
// library's "RCB write <attribute>" wrapping, which is pinned with its version.
func entryIDRefused(err error) bool {
	return errors.Is(err, mms.AccessObjectValueInvalid) && strings.Contains(err.Error(), "RCB write EntryID")
}

// disableRCB writes RptEna=false. The IED answers a write per item, so a
// refusal arrives as an item result, not as the call's error; either one
// means the block is still enabled.
func disableRCB(ctx context.Context, cli *client.Client, domain, item string) error {
	res, err := cli.MMS().Write(ctx, domain, []string{item + "$RptEna"}, []*mms.Value{mms.NewBool(false)})
	if err != nil {
		return err
	}
	if len(res) > 0 && res[0] != nil {
		return res[0]
	}
	return nil
}

// installReportDiagnostics logs reports that match no active subscription.
// Without it an IED whose RptID differs from what the RCB advertises simply
// looks dead; with it the mismatch is one log line away.
func installReportDiagnostics(conn *Iec61850Connection) func() {
	cli := conn.Client()
	if cli == nil {
		return func() {}
	}
	return cli.MMS().OnInformationReport(func(ir *mms.InformationReport) {
		if jslog.Level() < jslog.LevelDetailed || len(ir.Values) == 0 {
			return
		}
		rptID := ir.Values[0].Text()
		conn.mu.Lock()
		_, known := conn.RcbByRptID[rptID]
		conn.mu.Unlock()
		if !known {
			jslog.Log(jslog.LevelDetailed, "%s Unmatched report RptID '%s'", conn.Name, rptID)
		}
	})
}

func containsString(list []string, s string) bool {
	t := strings.TrimSpace(s)
	for _, e := range list {
		if strings.TrimSpace(e) == t {
			return true
		}
	}
	return false
}
