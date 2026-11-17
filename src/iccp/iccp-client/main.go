/*
 * ICCP/TASE.2 Client Driver for JSON-SCADA
 * {json:scada} - Copyright (c) 2020-present - Ricardo L. Olsen
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
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/tase2/mms"
	"github.com/riclolsen/tase2/tase2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// tagMapping maps a MongoDB tag to an ICCP ObjectRef.
type tagMapping struct {
	tag rtData
	ref tase2.ObjectRef
}

// refKey is the "Domain/Item" key of an ObjectRef.
func refKey(ref tase2.ObjectRef) string {
	return ref.Domain + "/" + ref.Item
}

// activeClient holds the live TASE2 client for a connection (for command forwarding).
type activeClient struct {
	mu       sync.Mutex
	client   *tase2.Client
	endpoint *tase2.Endpoint
	connName string
}

var (
	activeClients   = make(map[int]*activeClient)
	activeClientsMu sync.Mutex
)

func setActiveClient(connNumber int, ac *activeClient) {
	activeClientsMu.Lock()
	activeClients[connNumber] = ac
	activeClientsMu.Unlock()
}

func getActiveClient(connNumber int) *activeClient {
	activeClientsMu.Lock()
	defer activeClientsMu.Unlock()
	return activeClients[connNumber]
}

// removeActiveClient unregisters ac, unless a newer client of the same
// connection has replaced it already.
func removeActiveClient(connNumber int, ac *activeClient) {
	activeClientsMu.Lock()
	if activeClients[connNumber] == ac {
		delete(activeClients, connNumber)
	}
	activeClientsMu.Unlock()
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("%s Version %s", DriverMsg, DriverVersion)
	log.Println("Usage: iccp-client [instance number] [log level] [config file name]")

	cfg, instanceNumber, instLogLevel := readConfigFile()
	setLogLevel(instLogLevel)
	configureTASE2Logging(currentLogLevel())
	maxTPDUSizeParam := readTuningInt(EnvPrefix+"MAX_TPDU_SIZE_PARAM", cfg.ICCPMaxTPDUSizeParam, 16)
	if maxTPDUSizeParam < 7 || maxTPDUSizeParam > 16 {
		LogMsg(LogLevelMin, "Config - Invalid max TPDU size parameter %d, using 16", maxTPDUSizeParam)
		maxTPDUSizeParam = 16
	}

	LogMsg(LogLevelMin, "Config - %s Version %s", DriverMsg, DriverVersion)
	LogMsg(LogLevelMin, "Config - Instance: %d", instanceNumber)
	LogMsg(LogLevelMin, "Config - Log level: %d", currentLogLevel())
	LogMsg(LogLevelMin, "Config - Max TPDU size parameter: %d", maxTPDUSizeParam)

	// Channel for data updates to MongoDB writer
	dataChan := make(chan dataUpdate, 50000)

	// Connect once. The MongoDB driver reconnects by itself, and every
	// goroutine started below retries its own operations, so the driver is
	// started exactly once: restarting it on a lost ping would duplicate the
	// writers, the command watcher and the ICCP associations.
	var clientMongo *mongo.Client
	for {
		var err error
		clientMongo, err = mongoConnect(cfg)
		if err == nil {
			break
		}
		LogMsg(LogLevelMin, "MongoDB - Connection error: %v", err)
		time.Sleep(5 * time.Second)
	}
	LogMsg(LogLevelMin, "MongoDB - Connected correctly to MongoDB server")

	db := clientMongo.Database(cfg.MongoDatabaseName)
	collectionInstances := db.Collection("protocolDriverInstances")
	collectionConnections := db.Collection("protocolConnections")
	collectionCommands := db.Collection("commandsQueue")
	collectionRtData := db.Collection("realtimeData")

	// Verify instance config exists
	inst, err := getInstanceConfig(collectionInstances, instanceNumber)
	if err != nil {
		LogMsg(LogLevelMin, "Config - %v", err)
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}

	// Read connections
	connections, err := getConnections(collectionConnections, instanceNumber)
	if err != nil {
		LogMsg(LogLevelMin, "Config - %v", err)
		time.Sleep(5 * time.Second)
		os.Exit(2)
	}

	// Start MongoDB writer goroutine
	go mongoWriter(collectionRtData, dataChan)

	// Start command change stream watcher
	go commandWatcher(collectionCommands, collectionRtData, connections)

	// Start per-connection ICCP client goroutines
	for _, conn := range connections {
		go runICCPConnection(conn, collectionRtData, collectionConnections, cfg, dataChan, maxTPDUSizeParam)
	}

	// Start redundancy keepalive loop
	go func() {
		for {
			processRedundancy(collectionInstances, inst.ID, cfg)
			time.Sleep(5 * time.Second)
		}
	}()

	LogMsg(LogLevelMin, "ICCP - All connections started.")

	for {
		time.Sleep(5 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := clientMongo.Ping(ctx, nil)
		cancel()
		if err != nil {
			LogMsg(LogLevelMin, "MongoDB - Connection lost (driver keeps retrying): %v", err)
		}
	}
}

// connRuntime is the per-connection state that outlives one association: it
// publishes updates to the MongoDB writer and remembers the last update of
// each address, so tags can be invalidated (keeping their value) when data
// stops arriving.
type connRuntime struct {
	conn     protocolConnection
	dataChan chan<- dataUpdate

	mu   sync.Mutex
	last map[string]dataUpdate // by protocolSourceObjectAddress
}

func newConnRuntime(conn protocolConnection, dataChan chan<- dataUpdate) *connRuntime {
	return &connRuntime{conn: conn, dataChan: dataChan, last: make(map[string]dataUpdate)}
}

// publish queues an update for MongoDB. Nothing is written while this node is
// not the active one (the active node owns realtimeData). It returns false
// when the update was not queued.
func (r *connRuntime) publish(upd dataUpdate) bool {
	if !isActive.Load() {
		return false
	}
	upd.protocolSourceConnectionNumber = float64(r.conn.ProtocolConnectionNumber)
	select {
	case r.dataChan <- upd:
	default:
		return false
	}
	r.mu.Lock()
	r.last[upd.protocolSourceObjectAddress] = upd
	r.mu.Unlock()
	return true
}

// seed records the last raw value stored in realtimeData for tags not seen
// yet, so they can be invalidated before any value is received.
func (r *connRuntime) seed(tags []rtData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tags {
		addr := objAddrToString(t.ProtocolSourceObjectAddress)
		if _, seen := r.last[addr]; seen || t.SourceDataUpdate == nil || t.SourceDataUpdate.ValueAtSource == nil {
			continue
		}
		r.last[addr] = dataUpdate{
			protocolSourceObjectAddress: addr,
			value:                       *t.SourceDataUpdate.ValueAtSource,
			valueString:                 t.SourceDataUpdate.ValueStringAtSource,
			valueJson:                   t.SourceDataUpdate.ValueJsonAtSource,
			asdu:                        asduToString(t.ProtocolSourceASDU),
			invalid:                     t.SourceDataUpdate.InvalidAtSource,
		}
	}
}

// invalidate re-publishes the last value of the given addresses (all known
// addresses when addrs is nil) flagged invalid. Addresses already invalid or
// never seen are skipped. Returns how many were invalidated.
func (r *connRuntime) invalidate(addrs []string) int {
	now := time.Now()
	var pending []dataUpdate
	r.mu.Lock()
	if addrs == nil {
		for _, u := range r.last {
			if !u.invalid {
				pending = append(pending, u)
			}
		}
	} else {
		for _, a := range addrs {
			if u, ok := r.last[a]; ok && !u.invalid {
				pending = append(pending, u)
			}
		}
	}
	r.mu.Unlock()

	n := 0
	for _, u := range pending {
		u.invalid = true
		u.timeTag = now
		u.timeTagAtSource = time.Time{}
		u.timeTagAtSourceOk = false
		if r.publish(u) {
			n++
		}
	}
	return n
}

// runICCPConnection manages an ICCP connection lifecycle: connect, poll, subscribe, reconnect.
func runICCPConnection(conn protocolConnection, collectionRtData *mongo.Collection, collectionConnections *mongo.Collection, cfg configData, dataChan chan<- dataUpdate, maxTPDUSizeParam int) {
	connNumber := conn.ProtocolConnectionNumber
	rt := newConnRuntime(conn, dataChan)
	endpointIdx := 0

	for {
		if !protocolRunning() {
			time.Sleep(2 * time.Second)
			continue
		}

		// Read tags assigned to this connection
		tags, err := getTagsForConnection(collectionRtData, connNumber)
		if err != nil {
			LogMsg(LogLevelMin, "ICCP %s - Error reading tags: %v", conn.Name, err)
			time.Sleep(10 * time.Second)
			continue
		}

		if len(tags) == 0 {
			LogMsg(LogLevelDetailed, "ICCP %s - No tags found; waiting...", conn.Name)
			time.Sleep(10 * time.Second)
			continue
		}
		rt.seed(tags)

		// Build tag mappings (MongoDB tag -> ICCP ObjectRef)
		mappings := buildTagMappings(tags)
		if len(mappings) == 0 {
			LogMsg(LogLevelMin, "ICCP %s - No valid tag mappings; waiting...", conn.Name)
			time.Sleep(10 * time.Second)
			continue
		}

		LogMsg(LogLevelNormal, "ICCP %s - Loaded %d tags (%d mappable)", conn.Name, len(tags), len(mappings))

		// Get server endpoint from config: try the configured endpoints in
		// turn, moving on to the next one after a failed connect.
		if len(conn.EndpointURLs) == 0 {
			LogMsg(LogLevelMin, "ICCP %s - No endpoint URLs configured!", conn.Name)
			time.Sleep(30 * time.Second)
			continue
		}
		endpointIdx %= len(conn.EndpointURLs)
		host, port := parseHostPort(conn.EndpointURLs[endpointIdx])

		// Set up AP titles
		localAP := defaultString(conn.LocalApTitle, fmt.Sprintf("1.1.999.%d", connNumber))
		localAE := defaultInt(conn.LocalAeQualifier, 12)
		remoteAP := defaultString(conn.RemoteApTitle, "1.1.999.1")
		remoteAE := defaultInt(conn.RemoteAeQualifier, 12)

		// Create TASE2 client
		ep := tase2.NewEndpoint(tase2.EndpointActive)
		ep.SetLocalAPTitle(localAP, localAE)
		ep.SetRemoteAPTitle(remoteAP, remoteAE)
		ep.SetMaxTPDUSizeParam(maxTPDUSizeParam)
		if conn.TimeoutMs > 0 {
			ep.SetConnectTimeout(time.Duration(conn.TimeoutMs) * time.Millisecond)
		}
		// Set before Connect so the handshake is attributed to the connection.
		ep.SetLogHandler(tase2LogHandler(conn.Name))
		if conn.Password != "" {
			ep.SetAuthenticationPassword(conn.Password)
		}

		// Secure ICCP (IEC 62351-3): TLS wraps the whole association.
		if conn.UseSecurity {
			if err := ep.EnableTLS(conn.LocalCertFilePath, conn.PrivateKeyFilePath, conn.RootCertFilePath, false); err != nil {
				LogMsg(LogLevelMin, "ICCP %s - TLS setup failed: %v", conn.Name, err)
				time.Sleep(30 * time.Second)
				continue
			}
			if conn.AutoAcceptUntrusted || !conn.ChainValidation {
				ep.SetTLSInsecureSkipVerify(true)
			}
			LogMsg(LogLevelNormal, "ICCP %s - TLS enabled (chain validation: %v)", conn.Name, conn.ChainValidation && !conn.AutoAcceptUntrusted)
		}

		client := tase2.NewClient(ep)
		if conn.TimeoutMs > 0 {
			client.MMSClient.SetResponseTimeout(time.Duration(conn.TimeoutMs) * time.Millisecond)
		}
		// A link loss ends the association at once instead of at the next
		// poll tick.
		lost := make(chan error, 1)
		client.OnConnectionLost(func(err error) {
			select {
			case lost <- err:
			default:
			}
		})

		LogMsg(LogLevelNormal, "ICCP %s - Connecting to %s:%d (AP: %s)", conn.Name, host, port, remoteAP)
		if err := ep.Connect(host, port); err != nil {
			LogMsg(LogLevelMin, "ICCP %s - Connect failed: %v", conn.Name, err)
			updateConnectionStats(collectionConnections, conn, cfg, false)
			if n := rt.invalidate(nil); n > 0 {
				LogMsg(LogLevelNormal, "ICCP %s - Invalidated %d tag(s)", conn.Name, n)
			}
			endpointIdx++
			time.Sleep(10 * time.Second)
			continue
		}

		// Register the active client for command forwarding
		ac := &activeClient{
			client:   client,
			endpoint: ep,
			connName: conn.Name,
		}
		setActiveClient(connNumber, ac)

		// Read peer identity
		if vendor, model, rev, err := client.ReadPeerIdentity(); err == nil {
			LogMsg(LogLevelNormal, "ICCP %s - Peer: vendor=%q model=%q revision=%q", conn.Name, vendor, model, rev)
		} else {
			LogMsg(LogLevelDetailed, "ICCP %s - Peer identity not available: %v", conn.Name, err)
		}

		// Discover domains
		domains, err := client.ListDomains()
		if err != nil {
			LogMsg(LogLevelMin, "ICCP %s - Cannot list domains: %v", conn.Name, err)
			removeActiveClient(connNumber, ac)
			client.Close()
			rt.invalidate(nil)
			time.Sleep(10 * time.Second)
			continue
		}
		LogMsg(LogLevelNormal, "ICCP %s - Discovered %d domain(s): %v", conn.Name, len(domains), domains)

		// Build a domain→item lookup from our mappings for DSTS value handling
		mappingByRef := make(map[string]*tagMapping)
		for i := range mappings {
			mappingByRef[refKey(mappings[i].ref)] = &mappings[i]
		}

		// Periodically poll for data (integrity reads)
		pollInterval := 30
		if conn.GiInterval > 0 {
			pollInterval = int(conn.GiInterval)
		}

		// Try to set up DSTS subscriptions (one per domain) for live updates.
		// Points in a subscribed dataset get change + integrity reports from
		// the server and are excluded from the client-side poll.
		subs, covered := setupDSTSSubscriptions(client, rt, domains, mappings, mappingByRef, pollInterval)

		// Do initial poll immediately (all tags, so covered points also get
		// an initial value before the first DSTS report arrives)
		performPoll(client, rt, mappings, nil)

		pollTicker := time.NewTicker(time.Duration(pollInterval) * time.Second)
		redundancyTicker := time.NewTicker(2 * time.Second)

		// Stats update ticker (published once now, so the link shows up at once)
		updateConnectionStats(collectionConnections, conn, cfg, true)
		statsTicker := time.NewTicker(30 * time.Second)

		// Main poll loop
		connected := true
		for connected {
			select {
			case err := <-lost:
				LogMsg(LogLevelMin, "ICCP %s - Connection lost: %v", conn.Name, err)
				connected = false

			case <-redundancyTicker.C:
				if !protocolRunning() {
					LogMsg(LogLevelMin, "ICCP %s - Node inactive, closing association", conn.Name)
					connected = false
				}

			case <-pollTicker.C:
				if !isActive.Load() {
					continue
				}
				if _, _, _, err := client.ReadPeerIdentity(); err != nil {
					LogMsg(LogLevelMin, "ICCP %s - Connectivity check failed: %v", conn.Name, err)
					connected = false
					break
				}
				performPoll(client, rt, mappings, covered)

			case <-statsTicker.C:
				updateConnectionStats(collectionConnections, conn, cfg, true)
			}
		}

		pollTicker.Stop()
		redundancyTicker.Stop()
		statsTicker.Stop()

		// Cleanup: deactivate transfer sets (best effort), then Close (stops
		// the MMS receive loop and disconnects)
		removeActiveClient(connNumber, ac)
		if client.IsConnected() {
			for _, s := range subs {
				if err := client.StopDSTransferSet(s.domain, s.slot); err != nil {
					LogMsg(LogLevelDetailed, "ICCP %s - DSTS stop %s/%s: %v", conn.Name, s.domain, s.slot, err)
				}
			}
		}
		client.Close()
		updateConnectionStats(collectionConnections, conn, cfg, false)
		if n := rt.invalidate(nil); n > 0 {
			LogMsg(LogLevelNormal, "ICCP %s - Invalidated %d tag(s)", conn.Name, n)
		}
		LogMsg(LogLevelNormal, "ICCP %s - Disconnected, reconnecting in 10s...", conn.Name)
		time.Sleep(10 * time.Second)
	}
}

// buildTagMappings converts MongoDB tags to ICCP ObjectRefs.
func buildTagMappings(tags []rtData) []tagMapping {
	var mappings []tagMapping
	for _, t := range tags {
		addr := strings.TrimSpace(objAddrToString(t.ProtocolSourceObjectAddress))
		if addr == "" {
			continue
		}

		parts := strings.SplitN(addr, "/", 2)
		if len(parts) != 2 {
			LogMsg(LogLevelDetailed, "TagMap - Invalid address format for %s: %s (expected Domain/Item)", t.Tag, addr)
			continue
		}

		mappings = append(mappings, tagMapping{
			tag: t,
			ref: tase2.ObjectRef{Domain: parts[0], Item: parts[1]},
		})
	}
	return mappings
}

// pollChunkSize limits how many object refs go into a single MMS Read request,
// to stay within the negotiated PDU size on large tag sets.
const pollChunkSize = 100

// performPoll reads mapped tags and publishes the updates. Points whose
// "Domain/Item" key is in skip (covered by an active DSTS subscription) are
// not polled; pass nil to poll everything. A point the server fails to
// return is invalidated.
func performPoll(client *tase2.Client, rt *connRuntime, mappings []tagMapping, skip map[string]bool) {
	if !isActive.Load() {
		return
	}
	var polled []tagMapping
	for _, m := range mappings {
		if skip[refKey(m.ref)] {
			continue
		}
		polled = append(polled, m)
	}
	if len(polled) == 0 {
		return
	}

	count, failed, dropped := 0, 0, 0
	for start := 0; start < len(polled); start += pollChunkSize {
		end := start + pollChunkSize
		if end > len(polled) {
			end = len(polled)
		}
		chunk := polled[start:end]

		refs := make([]tase2.ObjectRef, len(chunk))
		for i, m := range chunk {
			refs[i] = m.ref
		}

		vals, err := client.ReadMultiple(refs)
		if err != nil {
			LogMsg(LogLevelMin, "ICCP %s - Poll ReadMultiple error: %v", rt.conn.Name, err)
			return
		}

		now := time.Now()
		var failedAddrs []string
		for i, m := range chunk {
			if i >= len(vals) || vals[i] == nil {
				failedAddrs = append(failedAddrs, objAddrToString(m.tag.ProtocolSourceObjectAddress))
				continue
			}
			if rt.publish(dataValueToUpdate(vals[i], m, now, rt.conn.HoursShift)) {
				count++
			} else {
				dropped++
			}
		}
		if len(failedAddrs) > 0 {
			failed += len(failedAddrs)
			rt.invalidate(failedAddrs)
		}
	}

	if dropped > 0 {
		LogMsg(LogLevelMin, "ICCP %s - Data channel full, discarded %d polled value(s)!", rt.conn.Name, dropped)
	}
	if failed > 0 {
		LogMsg(LogLevelDetailed, "ICCP %s - Poll: %d point(s) not returned by the server", rt.conn.Name, failed)
	}
	if count > 0 {
		LogMsg(LogLevelDetailed, "ICCP %s - Polled %d/%d values", rt.conn.Name, count, len(polled))
	}
}

// dstsSub identifies an activated DSTS subscription (for teardown).
type dstsSub struct {
	domain string
	slot   string
}

// jsScadaDataSetName is the domain-scope dataset the driver creates on the
// server with exactly the mapped points of that domain.
const jsScadaDataSetName = "JSSCADA_DS"

// maxFallbackDataSets bounds how many existing datasets are inspected when
// the driver cannot create its own.
const maxFallbackDataSets = 50

// setupDSTSSubscriptions sets up one DSTS subscription per domain that has
// mapped tags, so the server reports changes (RBE) plus periodic integrity
// snapshots instead of the client polling. Returns the activated
// subscriptions and the "Domain/Item" keys of the points they report.
func setupDSTSSubscriptions(
	client *tase2.Client,
	rt *connRuntime,
	domains []string,
	mappings []tagMapping,
	mappingByRef map[string]*tagMapping,
	pollInterval int,
) ([]dstsSub, map[string]bool) {
	var subs []dstsSub
	covered := make(map[string]bool)
	if len(domains) == 0 {
		return subs, covered
	}

	connName := rt.conn.Name
	hoursShift := rt.conn.HoursShift

	// Register the report callback BEFORE activating any transfer set so the
	// initial integrity report is not missed. A single callback serves all
	// domain subscriptions; values are matched via mappingByRef.
	client.OnDSTransferSetReport(func(report tase2.Report, finished bool) {
		if !finished || !isActive.Load() {
			return
		}
		count, dropped := 0, 0
		var failedAddrs []string
		now := time.Now()
		for _, v := range report.Values {
			m, ok := mappingByRef[v.Domain+"/"+v.Item]
			if !ok {
				continue
			}
			if v.Value == nil {
				failedAddrs = append(failedAddrs, objAddrToString(m.tag.ProtocolSourceObjectAddress))
				continue
			}
			if rt.publish(dataValueToUpdate(v.Value, *m, now, hoursShift)) {
				count++
			} else {
				dropped++
			}
		}
		if len(failedAddrs) > 0 {
			rt.invalidate(failedAddrs)
		}
		if dropped > 0 {
			LogMsg(LogLevelMin, "ICCP %s - Data channel full, discarded %d value(s) of DSTS report #%d!",
				connName, dropped, report.SeqNum)
		}
		LogMsg(LogLevelDetailed, "ICCP %s - DSTS report #%d: %d values (%d mapped)",
			connName, report.SeqNum, len(report.Values), count)
	})

	// Group mapped refs by domain
	refsByDomain := make(map[string][]tase2.ObjectRef)
	for _, m := range mappings {
		refsByDomain[m.ref.Domain] = append(refsByDomain[m.ref.Domain], m.ref)
	}

	for _, domain := range domains {
		refs := refsByDomain[domain]
		if len(refs) == 0 {
			continue
		}

		tsRef, err := allocateDSTransferSet(client, domain)
		if err != nil {
			LogMsg(LogLevelDetailed, "ICCP %s - Domain %s: no DSTS available (%v); will poll", connName, domain, err)
			continue
		}

		// Prefer a dataset containing exactly the mapped points of this
		// domain. A leftover from an earlier association (servers may keep
		// domain-scope datasets) can hold an outdated member list, so it is
		// deleted first. Fall back to the existing dataset that holds the
		// most mapped points; only those points are then left to DSTS.
		dsRef := tase2.ObjectRef{Domain: domain, Item: jsScadaDataSetName}
		members := refs
		if err := client.DeleteDataSet(domain + "/" + jsScadaDataSetName); err == nil {
			LogMsg(LogLevelDetailed, "ICCP %s - Deleted stale dataset %s/%s", connName, domain, jsScadaDataSetName)
		}
		if err := client.CreateDataSet(domain, jsScadaDataSetName, refs); err != nil {
			LogMsg(LogLevelDetailed, "ICCP %s - CreateDataSet %s/%s: %v", connName, domain, jsScadaDataSetName, err)
			dsRef, members = fallbackDataSet(client, domain, refs)
			if len(members) == 0 {
				LogMsg(LogLevelDetailed, "ICCP %s - No usable dataset for domain %s; will poll", connName, domain)
				continue
			}
		}

		dstsCfg := tase2.DSTSConfig{
			DataSet:        dsRef,
			BufferTime:     1,
			IntegrityCheck: pollInterval,
			RBE:            true,
			Conditions:     tase2.DSConditionChange | tase2.DSConditionIntegrity,
		}
		if err := client.ConfigureDSTransferSet(tsRef.Domain, tsRef.Item, dstsCfg); err != nil {
			LogMsg(LogLevelMin, "ICCP %s - DSTS configure %s: %v", connName, refKey(tsRef), err)
			continue
		}
		if err := client.StartDSTransferSet(tsRef.Domain, tsRef.Item); err != nil {
			LogMsg(LogLevelMin, "ICCP %s - DSTS activate %s: %v", connName, refKey(tsRef), err)
			continue
		}

		dsLabel := dsRef.Item
		if dsRef.Domain != "" {
			dsLabel = refKey(dsRef)
		}
		LogMsg(LogLevelNormal, "ICCP %s - DSTS subscription activated on %s -> %s (%d of %d mapped points)",
			connName, refKey(tsRef), dsLabel, len(members), len(refs))
		subs = append(subs, dstsSub{domain: tsRef.Domain, slot: tsRef.Item})
		for _, r := range members {
			covered[refKey(r)] = true
		}
	}

	return subs, covered
}

// allocateDSTransferSet obtains a DSTransferSet of a domain for this
// association: through Next_DSTransfer_Set as the standard requires, or, for
// servers without that pointer, the first transfer set the domain lists.
func allocateDSTransferSet(client *tase2.Client, domain string) (tase2.ObjectRef, error) {
	ref, err := client.ReadNextDSTransferSet(domain)
	if err == nil {
		return ref, nil
	}
	vars, listErr := client.ListDomainVariables(domain)
	if listErr != nil {
		return tase2.ObjectRef{}, err
	}
	if _, _, slots, _ := tase2.ClassifyDomainVariables(vars); len(slots) > 0 {
		return tase2.ObjectRef{Domain: domain, Item: slots[0]}, nil
	}
	return tase2.ObjectRef{}, err
}

// fallbackDataSet picks, among the existing domain- and VMD-scope datasets,
// the one holding the most of refs, and returns it with the refs it holds.
func fallbackDataSet(client *tase2.Client, domain string, refs []tase2.ObjectRef) (tase2.ObjectRef, []tase2.ObjectRef) {
	var candidates []tase2.ObjectRef
	if domSets, err := client.ListDomainDatasets(domain); err == nil {
		for _, n := range domSets {
			candidates = append(candidates, tase2.ObjectRef{Domain: domain, Item: strings.TrimPrefix(n, domain+"/")})
		}
	}
	if vmdSets, err := client.ListVMDDatasets(); err == nil {
		for _, n := range vmdSets {
			candidates = append(candidates, tase2.ObjectRef{Item: n})
		}
	}
	if len(candidates) > maxFallbackDataSets {
		candidates = candidates[:maxFallbackDataSets]
	}

	wanted := make(map[string]bool, len(refs))
	for _, r := range refs {
		wanted[refKey(r)] = true
	}
	var best tase2.ObjectRef
	var bestMembers []tase2.ObjectRef
	for _, c := range candidates {
		name := c.Item
		if c.Domain != "" {
			name = refKey(c)
		}
		members, err := client.GetDataSetMembers(name)
		if err != nil {
			continue
		}
		var held []tase2.ObjectRef
		for _, m := range members {
			if m.Domain == "" {
				m.Domain = domain
			}
			if wanted[refKey(m)] {
				held = append(held, m)
			}
		}
		if len(held) > len(bestMembers) {
			best, bestMembers = c, held
		}
	}
	return best, bestMembers
}

// dataValueToUpdate converts a TASE2 DataValue to a dataUpdate for MongoDB.
// ICCP typed points (RealQ, StateQTimeTag, ...Extended, etc.) are decoded into
// value + quality + source timestamp; plain MMS scalars are handled directly.
func dataValueToUpdate(dv *tase2.DataValue, m tagMapping, now time.Time, hoursShift float64) dataUpdate {
	upd := dataUpdate{
		protocolSourceObjectAddress: objAddrToString(m.tag.ProtocolSourceObjectAddress),
		asdu:                        asduToString(m.tag.ProtocolSourceASDU),
		timeTag:                     now,
		invalid:                     false,
	}

	if dv == nil {
		upd.invalid = true
		return upd
	}

	dp := tase2.DecodeICCPNamed(m.ref.Item, dv)

	switch {
	case dp.Real != nil:
		upd.value = *dp.Real
		upd.valueString = fmt.Sprintf("%f", *dp.Real)
	case dp.Discrete != nil:
		upd.value = float64(*dp.Discrete)
		upd.valueString = fmt.Sprintf("%d", *dp.Discrete)
	case dp.State != nil:
		// JSON-SCADA digitals are 0/1: ICCP On (closed) is 1, Off (open) 0.
		// The in-between and invalid states keep 0 flagged invalid, the
		// in-between one also transient (as IEC double points are).
		upd.valueString = dp.State.String()
		switch *dp.State {
		case tase2.StateOn:
			upd.value = 1
		case tase2.StateOff:
			upd.value = 0
		case tase2.StateIntermediate:
			upd.invalid = true
			upd.transient = true
		default:
			upd.invalid = true
		}
	default:
		// Not an ICCP typed point: plain MMS scalar handling
		switch {
		case dv.BoolVal != nil:
			if *dv.BoolVal {
				upd.value = 1
				upd.valueString = "true"
			} else {
				upd.value = 0
				upd.valueString = "false"
			}
			upd.valueJson = fmt.Sprintf("%v", *dv.BoolVal)
		case dv.UnsignedVal != nil:
			upd.value = float64(*dv.UnsignedVal)
			upd.valueString = fmt.Sprintf("%d", *dv.UnsignedVal)
		case dv.VisibleStringVal != "":
			upd.valueString = dv.VisibleStringVal
			if v, err := strconv.ParseFloat(dv.VisibleStringVal, 64); err == nil {
				upd.value = v
			}
			upd.valueJson = fmt.Sprintf("%q", dv.VisibleStringVal)
		case dv.BitStringVal != nil:
			upd.valueString = fmt.Sprintf("%x", dv.BitStringVal)
		case dv.OctetStringVal != nil:
			upd.valueString = fmt.Sprintf("%x", dv.OctetStringVal)
		default:
			upd.valueString = dv.String()
		}
	}

	if dp.Quality != nil {
		switch dp.Quality.Validity {
		case tase2.QualityInvalid:
			upd.invalid = true
		case tase2.QualityHeld, tase2.QualitySuspect:
			upd.notTopical = true
		}
		// Any current source other than telemetered means the value did not come
		// straight from the process: entered (manually substituted), calculated
		// or estimated all map to substitutedAtSource.
		switch dp.Quality.Source {
		case tase2.SourceEntered, tase2.SourceCalculated, tase2.SourceEstimated:
			upd.substituted = true
		}
	}

	if t, ok := dp.Time(); ok {
		upd.timeTagAtSource = iccpTimeStampToTime(t, hoursShift)
		upd.timeTagAtSourceOk = true
	}

	return upd
}

// iccpTimeStampToTime applies the connection's hoursShift to a decoded ICCP
// time stamp. The time is already absolute (GMTBasedS seconds, plus
// milliseconds for the 802 Ed.2 QTimeTagExtended types); hoursShift only
// compensates a peer that stamps local time instead of UTC.
func iccpTimeStampToTime(t time.Time, hoursShift float64) time.Time {
	return t.Add(time.Duration(hoursShift * float64(time.Hour)))
}

// updateConnectionStats updates the stats field on the protocol connection document.
func updateConnectionStats(collectionConnections *mongo.Collection, conn protocolConnection, cfg configData, isConnected bool) {
	filter := bson.D{
		{Key: "protocolConnectionNumber", Value: conn.ProtocolConnectionNumber},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "stats", Value: bson.D{
				{Key: "nodeName", Value: cfg.NodeName},
				{Key: "timeTag", Value: time.Now()},
				{Key: "isConnected", Value: isConnected},
			}},
		}},
	}
	_, err := collectionConnections.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		LogMsg(LogLevelDetailed, "ICCP %s - Stats update error: %v", conn.Name, err)
	}
}

// mongoWriter receives data updates and bulk-writes them to MongoDB realtimeData.
func mongoWriter(collectionRtData *mongo.Collection, dataChan <-chan dataUpdate) {
	batch := make([]mongo.WriteModel, 0, 5000)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case upd := <-dataChan:
			updOper := buildSourceDataUpdate(upd)
			batch = append(batch, updOper)

			if len(batch) >= 5000 {
				flushBatch(collectionRtData, batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				flushBatch(collectionRtData, batch)
				batch = batch[:0]
			}
		}
	}
}

// buildSourceDataUpdate creates a MongoDB update operation for a dataUpdate.
func buildSourceDataUpdate(upd dataUpdate) mongo.WriteModel {
	filter := bson.D{
		{Key: "protocolSourceConnectionNumber", Value: upd.protocolSourceConnectionNumber},
		{Key: "protocolSourceObjectAddress", Value: upd.protocolSourceObjectAddress},
	}

	asduAtSource := upd.asdu
	if asduAtSource == "" {
		asduAtSource = "unknown"
	}
	// No source time: store null rather than the zero time (year 1).
	var timeTagAtSource interface{}
	if upd.timeTagAtSourceOk {
		timeTagAtSource = upd.timeTagAtSource
	}

	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "sourceDataUpdate", Value: bson.D{
				{Key: "valueAtSource", Value: upd.value},
				{Key: "valueStringAtSource", Value: upd.valueString},
				{Key: "valueJsonAtSource", Value: upd.valueJson},
				{Key: "invalidAtSource", Value: upd.invalid},
				{Key: "notTopicalAtSource", Value: upd.notTopical},
				{Key: "substitutedAtSource", Value: upd.substituted},
				{Key: "blockedAtSource", Value: false},
				{Key: "overflowAtSource", Value: false},
				{Key: "transientAtSource", Value: upd.transient},
				{Key: "carryAtSource", Value: false},
				{Key: "asduAtSource", Value: asduAtSource},
				{Key: "causeOfTransmissionAtSource", Value: 20},
				{Key: "timeTag", Value: upd.timeTag},
				{Key: "timeTagAtSource", Value: timeTagAtSource},
				{Key: "timeTagAtSourceOk", Value: upd.timeTagAtSourceOk},
			}},
		}},
	}

	return mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
}

// flushBatch writes a batch of updates to MongoDB.
func flushBatch(collectionRtData *mongo.Collection, batch []mongo.WriteModel) {
	if len(batch) == 0 {
		return
	}
	res, err := collectionRtData.BulkWrite(
		context.Background(),
		batch,
		options.BulkWrite().SetOrdered(false),
	)
	if err != nil {
		LogMsg(LogLevelMin, "MongoDB - Bulk write error: %v", err)
		return
	}
	if res != nil {
		LogMsg(LogLevelDetailed, "MongoDB - Bulk write: %d matched, %d modified",
			res.MatchedCount, res.ModifiedCount)
	}
}

// commandWatcher monitors the commandsQueue collection and forwards commands to ICCP servers.
func commandWatcher(collectionCommands *mongo.Collection, collectionRtData *mongo.Collection, connections []protocolConnection) {
	connMap := make(map[int]*protocolConnection)
	for i := range connections {
		conn := connections[i]
		connMap[conn.ProtocolConnectionNumber] = &conn
	}

	for {
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.D{{Key: "operationType", Value: "insert"}}}},
		}

		opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
		cs, err := collectionCommands.Watch(context.TODO(), pipeline, opts)
		if err != nil {
			LogMsg(LogLevelMin, "Commands CS - Error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		LogMsg(LogLevelNormal, "Commands CS - Watching for commands...")

		for cs.Next(context.TODO()) {
			if !isActive.Load() {
				continue
			}

			var event insertChange
			if err := cs.Decode(&event); err != nil {
				LogMsg(LogLevelMin, "Commands CS - Decode error: %v", err)
				continue
			}

			cmd := event.FullDocument
			connNumber := int(cmd.ProtocolSourceConnectionNumber)

			conn, ok := connMap[connNumber]
			if !ok {
				// The change stream sees every command in the queue, including
				// those of other drivers and instances. A connection this driver
				// does not own is not ours to cancel: just log it.
				LogMsg(LogLevelDetailed, "Commands CS - Connection %d not handled by this driver, ignoring command", connNumber)
				continue
			}
			if !conn.CommandsEnabled {
				LogMsg(LogLevelMin, "Commands CS - Commands disabled for %s: %s", conn.Name, cmd.Tag)
				cancelCommand(collectionCommands, cmd.ID, "commands disabled")
				continue
			}

			// Check for expired command
			if time.Since(cmd.TimeTag) > 10*time.Second {
				LogMsg(LogLevelMin, "Commands CS - Command expired for %s: %s", conn.Name, cmd.Tag)
				cancelCommand(collectionCommands, cmd.ID, "expired")
				continue
			}

			// Look up the command tag in realtimeData to get the ICCP address
			var cmdTag rtData
			filter := bson.D{
				{Key: "tag", Value: cmd.Tag},
				{Key: "protocolSourceConnectionNumber", Value: float64(connNumber)},
				{Key: "origin", Value: "command"},
			}
			err := collectionRtData.FindOne(context.TODO(), filter).Decode(&cmdTag)
			if err != nil {
				LogMsg(LogLevelMin, "Commands CS - Cannot find command tag %s: %v", cmd.Tag, err)
				cancelCommand(collectionCommands, cmd.ID, "tag not found")
				continue
			}

			// Parse ICCP address
			addr := strings.TrimSpace(objAddrToString(cmdTag.ProtocolSourceObjectAddress))
			parts := strings.SplitN(addr, "/", 2)
			if len(parts) != 2 {
				LogMsg(LogLevelMin, "Commands CS - Invalid ICCP address for %s: %s", cmd.Tag, addr)
				cancelCommand(collectionCommands, cmd.ID, "invalid ICCP address")
				continue
			}
			domain, item := parts[0], parts[1]

			if cmdTag.ProtocolSourceCommandUseSBO {
				LogMsg(LogLevelMin, "Commands CS - SBO requested for %s but select-before-operate is not supported; sending direct operate", cmd.Tag)
			}

			// Convert command value to TASE2 DataValue
			dv := commandToDataValue(cmd)

			// Try to use the active client for this connection
			ac := getActiveClient(connNumber)
			if ac != nil {
				ac.mu.Lock()
				if ac.client != nil {
					dv = adaptCommandValue(ac.client, domain, item, dv)
					err := ac.client.WriteDataValue(domain, item, dv)
					ac.mu.Unlock()
					if err != nil {
						LogMsg(LogLevelMin, "Commands CS - Write failed for %s/%s: %v", domain, item, err)
						cancelCommand(collectionCommands, cmd.ID, err.Error())
					} else {
						LogMsg(LogLevelNormal, "Commands CS - Command delivered via %s: %s/%s = %v",
							ac.connName, domain, item, dv.String())
						deliverCommand(collectionCommands, cmd.ID, true, "OK")
					}
				} else {
					ac.mu.Unlock()
					LogMsg(LogLevelMin, "Commands CS - Client nil for %s, cannot send command", conn.Name)
					cancelCommand(collectionCommands, cmd.ID, "no active ICCP connection")
				}
			} else {
				LogMsg(LogLevelMin, "Commands CS - No active client for %s, cannot send command", conn.Name)
				cancelCommand(collectionCommands, cmd.ID, "no active ICCP connection")
			}
		}

		if err := cs.Err(); err != nil {
			LogMsg(LogLevelMin, "Commands CS - Stream error: %v", err)
		}
		cs.Close(context.TODO())
		time.Sleep(5 * time.Second)
	}
}

// commandToDataValue converts a command queue entry to a TASE2 DataValue.
func commandToDataValue(cmd commandQueueEntry) *tase2.DataValue {
	asdu := strings.ToLower(asduToString(cmd.ProtocolSourceASDU))

	if strings.Count(asdu, "[") > 0 {
		return tase2.NewVisibleStringValue(cmd.ValueString)
	}

	switch asdu {
	case "boolean":
		return tase2.NewBooleanValue(cmd.Value != 0)
	case "sbyte":
		return tase2.NewIntegerValue(int64(int8(cmd.Value)))
	case "byte":
		return tase2.NewUnsignedValue(uint64(byte(cmd.Value)))
	case "int16":
		return tase2.NewIntegerValue(int64(int16(cmd.Value)))
	case "uint16":
		return tase2.NewUnsignedValue(uint64(uint16(cmd.Value)))
	case "int32", "integer":
		return tase2.NewIntegerValue(int64(int32(cmd.Value)))
	case "uint32":
		return tase2.NewUnsignedValue(uint64(uint32(cmd.Value)))
	case "int64":
		return tase2.NewIntegerValue(int64(cmd.Value))
	case "uint64":
		return tase2.NewUnsignedValue(uint64(cmd.Value))
	case "float":
		return tase2.NewFloat32Value(float32(cmd.Value))
	case "double":
		return tase2.NewFloat64Value(cmd.Value)
	case "string", "bytestring", "localizedtext", "qualifiedname", "nodeid", "guid":
		return tase2.NewVisibleStringValue(cmd.ValueString)
	default:
		return tase2.NewFloat64Value(cmd.Value)
	}
}

// adaptCommandValue coerces a command value to the type the server declares
// for the target control point, so a numeric command reaches a Real setpoint
// as float and a Command/Discrete point as integer regardless of the ASDU
// configured on the tag. If the type cannot be read, the value is unchanged.
func adaptCommandValue(client *tase2.Client, domain, item string, dv *tase2.DataValue) *tase2.DataValue {
	spec, err := client.GetVariableType(tase2.ObjectRef{Domain: domain, Item: item})
	if err != nil || spec == nil {
		return dv
	}

	num, hasNum := dataValueToFloat(dv)

	switch spec.Type {
	case mms.FloatingPoint:
		if dv.FloatVal == nil && hasNum {
			if spec.FormatWidth > 32 {
				return tase2.NewFloat64Value(num)
			}
			return tase2.NewFloat32Value(float32(num))
		}
	case mms.Integer:
		if dv.IntVal == nil && hasNum {
			return tase2.NewIntegerValue(int64(num))
		}
	case mms.Unsigned:
		if dv.UnsignedVal == nil && hasNum {
			return tase2.NewUnsignedValue(uint64(num))
		}
	case mms.Boolean:
		if dv.BoolVal == nil && hasNum {
			return tase2.NewBooleanValue(num != 0)
		}
	case mms.Structure:
		LogMsg(LogLevelMin, "Commands - Control point %s/%s is a structure; sending raw value (may be rejected)", domain, item)
	}
	return dv
}

// dataValueToFloat extracts a numeric value from a scalar DataValue.
func dataValueToFloat(dv *tase2.DataValue) (float64, bool) {
	switch {
	case dv == nil:
		return 0, false
	case dv.FloatVal != nil:
		return *dv.FloatVal, true
	case dv.IntVal != nil:
		return float64(*dv.IntVal), true
	case dv.UnsignedVal != nil:
		return float64(*dv.UnsignedVal), true
	case dv.BoolVal != nil:
		if *dv.BoolVal {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// parseHostPort splits an endpoint ("host", "host:port", "[ipv6]:port", a
// bare IPv6 address, optionally prefixed by a scheme such as "iccp://") into
// host and port. The port defaults to 102.
func parseHostPort(endpointURL string) (string, int) {
	s := strings.TrimSpace(endpointURL)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	s = strings.TrimSuffix(s, "/")
	host, port := strings.Trim(s, "[]"), 102
	if h, p, err := net.SplitHostPort(s); err == nil {
		host = h
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			port = n
		}
	}
	return host, port
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func defaultInt(i, def int) int {
	if i == 0 {
		return def
	}
	return i
}
