/*
 * ICCP/TASE.2 Server Driver for JSON-SCADA
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/tase2/tase2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// datasetDef holds a dataset name ("Domain/Name"), its domain and its member
// ObjectRefs.
type datasetDef struct {
	domain string
	name   string
	items  []tase2.ObjectRef
}

// serverEntry tracks a live per-connection TASE2 server in the registry.
type serverEntry struct {
	server     *tase2.Server
	connection protocolConnection
	endpoint   *tase2.Endpoint
	since      time.Time
	peer       tase2.EventPeer // set by the AssociationAccepted event
}

// serverRegistry tracks all live per-connection servers, and keeps the
// counters of associations that have ended so connection statistics survive
// client reconnects.
type serverRegistry struct {
	mu      sync.Mutex
	entries map[*tase2.Server]*serverEntry
	closed  map[int]*connStats // by protocolConnectionNumber
}

func newServerRegistry() *serverRegistry {
	return &serverRegistry{
		entries: make(map[*tase2.Server]*serverEntry),
		closed:  make(map[int]*connStats),
	}
}

func (r *serverRegistry) add(srv *tase2.Server, ep *tase2.Endpoint, conn protocolConnection) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[srv] = &serverEntry{server: srv, endpoint: ep, connection: conn, since: time.Now()}
	return len(r.entries)
}

func (r *serverRegistry) remove(srv *tase2.Server) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	// The per-connection transport is already closed by ServeClients before
	// the post-disconnect hook (which calls remove) runs; its counters are
	// final, so fold them into the connection totals.
	if e, ok := r.entries[srv]; ok {
		n := e.connection.ProtocolConnectionNumber
		if r.closed[n] == nil {
			r.closed[n] = newConnStats()
		}
		r.closed[n].add(srv.Stats())
		delete(r.entries, srv)
	}
	return len(r.entries)
}

// setPeer records the peer identity of a live association.
func (r *serverRegistry) setPeer(srv *tase2.Server, peer tase2.EventPeer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entries[srv]; ok {
		e.peer = peer
	}
}

func (r *serverRegistry) snapshot() []*serverEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*serverEntry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e)
	}
	return out
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("%s Version %s", DriverMsg, DriverVersion)
	log.Println("Usage: iccp-server [instance number] [log level] [config file name]")

	cfg, instanceNumber, instLogLevel := readConfigFile()
	setLogLevel(instLogLevel)
	configureTASE2Logging(currentLogLevel())
	getNameListMaxPerResponse := readTuningInt(EnvPrefix+"GETNAMELIST_MAX_PER_RESPONSE", cfg.ICCPGetNameListMaxPerResponse, 1000)
	getNameListMaxBytes := readTuningInt(EnvPrefix+"GETNAMELIST_MAX_BYTES", cfg.ICCPGetNameListMaxBytes, 60000)
	maxTPDUSizeParam := readTuningInt(EnvPrefix+"MAX_TPDU_SIZE_PARAM", cfg.ICCPMaxTPDUSizeParam, 16)
	datasetChunkSize := readTuningInt(EnvPrefix+"DATASET_CHUNK_SIZE", cfg.ICCPDatasetChunkSize, 0)
	maxNVLAttrsItems := readTuningSignedInt(EnvPrefix+"MAX_NVL_ATTRS_ITEMS", cfg.ICCPMaxNVLAttrsItems, 1750)
	if maxTPDUSizeParam < 7 || maxTPDUSizeParam > 16 {
		LogMsg(LogLevelMin, "Config - Invalid max TPDU size parameter %d, using 16", maxTPDUSizeParam)
		maxTPDUSizeParam = 16
	}

	LogMsg(LogLevelMin, "Config - %s Version %s", DriverMsg, DriverVersion)
	LogMsg(LogLevelMin, "Config - Instance: %d", instanceNumber)
	LogMsg(LogLevelMin, "Config - Log level: %d", currentLogLevel())
	LogMsg(LogLevelMin, "Config - GetNameList max identifiers: %d", getNameListMaxPerResponse)
	LogMsg(LogLevelMin, "Config - GetNameList max estimated bytes: %d", getNameListMaxBytes)
	LogMsg(LogLevelMin, "Config - Max TPDU size parameter: %d", maxTPDUSizeParam)
	if datasetChunkSize > 0 {
		LogMsg(LogLevelMin, "Config - Dataset chunk size: %d", datasetChunkSize)
	} else {
		LogMsg(LogLevelMin, "Config - Dataset chunking: disabled (single dataset per domain)")
	}

	// The driver starts once. Until it has, a failed step drops the MongoDB
	// client and retries; afterwards the MongoDB driver reconnects by itself
	// and every goroutine retries its own operations, so a lost ping is only
	// logged (restarting would duplicate listeners, watchers and loops).
	var clientMongo *mongo.Client
	for {
		if clientMongo == nil {
			var err error
			clientMongo, err = mongoConnect(cfg)
			if err != nil {
				LogMsg(LogLevelMin, "MongoDB - Connection error: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			LogMsg(LogLevelMin, "MongoDB - Connected correctly to MongoDB server")

			db := clientMongo.Database(cfg.MongoDatabaseName)
			collectionRtData := db.Collection("realtimeData")
			collectionInstances := db.Collection("protocolDriverInstances")
			collectionConnections := db.Collection("protocolConnections")
			collectionCommands := db.Collection("commandsQueue")

			// Verify instance config exists
			instance, err := getInstanceConfig(collectionInstances, instanceNumber)
			if err != nil {
				LogMsg(LogLevelMin, "Config - %v", err)
				clientMongo.Disconnect(context.TODO())
				clientMongo = nil
				time.Sleep(5 * time.Second)
				continue
			}

			// Read connections
			connections, err := getConnections(collectionConnections, instanceNumber)
			if err != nil {
				LogMsg(LogLevelMin, "Config - %v", err)
				clientMongo.Disconnect(context.TODO())
				clientMongo = nil
				time.Sleep(5 * time.Second)
				continue
			}

			// Union of the topics of all connections, for the MongoDB filters
			topicList := topicsOf(connections)

			// Query realtimeData for the tags to expose
			filter := bson.D{
				{Key: "_id", Value: bson.M{"$gt": 0}}, // exclude internal system data
			}
			if len(topicList) > 0 {
				// Restrict group1 to configured topics at the MongoDB query level
				filter = append(filter, bson.E{Key: "group1", Value: bson.M{"$in": topicList}})
				LogMsg(LogLevelNormal, "DataModel - Topic filter applied: %d unique group1(s)", len(topicList))
			}

			projection := bson.D{
				{Key: "_id", Value: 1},
				{Key: "tag", Value: 1},
				{Key: "type", Value: 1},
				{Key: "value", Value: 1},
				{Key: "valueString", Value: 1},
				{Key: "valueJson", Value: 1},
				{Key: "invalid", Value: 1},
				{Key: "transient", Value: 1},
				{Key: "frozen", Value: 1},
				{Key: "substituted", Value: 1},
				{Key: "description", Value: 1},
				{Key: "ungroupedDescription", Value: 1},
				{Key: "group1", Value: 1},
				{Key: "group2", Value: 1},
				{Key: "group3", Value: 1},
				{Key: "origin", Value: 1},
				{Key: "protocolSourceConnectionNumber", Value: 1},
				{Key: "protocolSourceObjectAddress", Value: 1},
				{Key: "protocolSourceASDU", Value: 1},
				{Key: "protocolSourceCommonAddress", Value: 1},
				{Key: "protocolSourceCommandDuration", Value: 1},
				{Key: "protocolSourceCommandUseSBO", Value: 1},
				{Key: "protocolSourceAccessLevel", Value: 1},
				{Key: "timeTagAtSource", Value: 1},
				{Key: "timeTagAtSourceOk", Value: 1},
				{Key: "commandBlocked", Value: 1},
				{Key: "supervisedOfCommand", Value: 1},
				{Key: "kconv1", Value: 1},
				{Key: "kconv2", Value: 1},
			}

			var allTags []rtData
			cursor, err := collectionRtData.Find(context.TODO(), filter, options.Find().SetProjection(projection))
			if err == nil {
				err = cursor.All(context.TODO(), &allTags)
				cursor.Close(context.TODO())
			}
			if err != nil {
				LogMsg(LogLevelMin, "MongoDB - Error reading realtimeData: %v", err)
				clientMongo.Disconnect(context.TODO())
				clientMongo = nil
				time.Sleep(5 * time.Second)
				continue
			}

			LogMsg(LogLevelNormal, "DataModel - Loaded %d tags from realtimeData", len(allTags))

			model := buildModel(allTags, connections, datasetChunkSize)
			populateRtDataCache(allTags, model.tagByKey)

			// Start servers for each connection
			registry := newServerRegistry()
			var listening []protocolConnection

			for _, conn := range connections {
				bindHost, port, err := parseLocalBind(conn.IPAddressLocalBind)
				if err != nil {
					LogMsg(LogLevelMin, "ICCP - Connection %q: invalid ipAddressLocalBind %q: %v; not listening",
						conn.Name, conn.IPAddressLocalBind, err)
					continue
				}

				localAPTitle := conn.LocalApTitle
				localAEQual := conn.LocalAeQualifier
				if localAPTitle == "" {
					localAPTitle = fmt.Sprintf("1.1.999.%d", conn.ProtocolConnectionNumber)
				}
				if localAEQual == 0 {
					localAEQual = 12
				}

				srvCfg := tase2.ServerConfig{
					Port:         port,
					LocalAPTitle: localAPTitle,
					LocalAEQual:  localAEQual,
					SupportedCBBs: []tase2.ConformanceBlock{
						tase2.CBB1_BasicPeriodicData,
						tase2.CBB2_ExtendedDataSets,
						tase2.CBB5_DeviceControl,
					},
					VendorName:                "JSON-SCADA",
					ModelName:                 "ICCP-Server",
					Revision:                  DriverVersion,
					AuthenticationPassword:    conn.Password,
					GetNameListMaxPerResponse: getNameListMaxPerResponse,
					GetNameListMaxBytes:       getNameListMaxBytes,
					MaxNVLAttrsItems:          maxNVLAttrsItems,
				}

				endpoint := tase2.NewEndpoint(tase2.EndpointPassive)
				endpoint.SetLocalAPTitle(localAPTitle, localAEQual)
				endpoint.SetMaxTPDUSizeParam(maxTPDUSizeParam)
				// Library messages for this listener's associations are
				// attributed to the connection; accepted endpoints inherit it.
				endpoint.SetLogHandler(tase2LogHandler(conn.Name))
				// The library binds passive endpoints to loopback unless told
				// otherwise; bind where ipAddressLocalBind says.
				if err := endpoint.SetListenAddress(bindHost); err != nil {
					LogMsg(LogLevelMin, "ICCP - Connection %q: %v; not listening", conn.Name, err)
					continue
				}

				// Secure ICCP (IEC 62351-3): TLS wraps the whole association.
				// chainValidation=true additionally requires and verifies
				// client certificates (mutual TLS) against rootCertFilePath.
				if conn.UseSecurity {
					if err := endpoint.EnableTLS(conn.LocalCertFilePath, conn.PrivateKeyFilePath,
						conn.RootCertFilePath, conn.ChainValidation); err != nil {
						LogMsg(LogLevelMin, "ICCP - Connection %q: TLS setup failed, not listening: %v", conn.Name, err)
						continue
					}
					LogMsg(LogLevelMin, "ICCP - Connection %q: TLS enabled (mutual: %v)", conn.Name, conn.ChainValidation)
				}

				if err := endpoint.Listen(port); err != nil {
					LogMsg(LogLevelMin, "ICCP - Cannot listen on port %d: %v", port, err)
					continue
				}

				LogMsg(LogLevelMin, "ICCP - Connection %q listening on %v (AP: %s, AE: %d)",
					conn.Name, endpoint.ListenAddr(), localAPTitle, localAEQual)

				// Serve clients: ServeClients runs the accept loop, one server
				// goroutine per association, and closes each transport when the
				// association ends; the post-disconnect hook keeps the registry
				// in sync for statistics and redundancy.
				listening = append(listening, conn)
				go func(ep *tase2.Endpoint, conn protocolConnection, srvCfg tase2.ServerConfig) {
					err := ep.ServeClients(serveClientCallbacks(registry, model.dataModel, srvCfg, conn, model.datasetDefs, collectionCommands))
					if errors.Is(err, net.ErrClosed) {
						LogMsg(LogLevelNormal, "ICCP - Listener closed for %s", conn.Name)
					} else if err != nil {
						LogMsg(LogLevelMin, "ICCP - Serve loop ended for %s: %v", conn.Name, err)
					}
				}(endpoint, conn, srvCfg)
			}

			// Publish per-connection association statistics
			go publishConnectionStats(collectionConnections, registry, listening, cfg.NodeName)

			// Start the MongoDB change stream watcher
			go watchRealtimeDataChanges(collectionRtData, model, ownConnections(connections), topicList)

			// Start redundancy keepalive loop. While this node is not to run
			// the protocol, new associations are refused (serveClientCallbacks)
			// and the live ones are aborted here.
			go func(id bson.ObjectID) {
				for {
					processRedundancy(collectionInstances, id, cfg)
					if !protocolRunning() {
						for _, e := range registry.snapshot() {
							LogMsg(LogLevelNormal, "ICCP - Node inactive, aborting association of %s", e.connection.Name)
							_ = e.server.Abort()
						}
					}
					time.Sleep(5 * time.Second)
				}
			}(instance.ID)

			LogMsg(LogLevelMin, "ICCP - All servers started. Waiting for connections...")
		}

		// Keep-alive: check MongoDB connection
		time.Sleep(5 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := clientMongo.Ping(ctx, nil)
		cancel()
		if err != nil {
			LogMsg(LogLevelMin, "MongoDB - Connection lost (driver keeps retrying): %v", err)
		}
	}
}

// ownConnections returns the numbers of this instance's connections; tags
// acquired by them are not served back.
func ownConnections(connections []protocolConnection) map[int]bool {
	nums := make(map[int]bool, len(connections))
	for _, c := range connections {
		nums[c.ProtocolConnectionNumber] = true
	}
	return nums
}

// topicsOf returns the union of the connections' topics (group1 values),
// sorted; empty when any connection has no topics (it serves every group1).
func topicsOf(connections []protocolConnection) []string {
	set := make(map[string]bool)
	for _, c := range connections {
		n := 0
		for _, t := range c.Topics {
			if t = strings.TrimSpace(t); t != "" {
				set[t] = true
				n++
			}
		}
		if n == 0 {
			return nil
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// domainNameOf returns the ICCP domain name of a group1 value: an MMS
// identifier, sanitized the same way as point names.
func domainNameOf(group1 string) string {
	return sanitizePointName(strings.TrimSpace(group1))
}

// topicAllowsDomain reports whether a connection's topics include the ICCP
// domain (no topics: every domain).
func topicAllowsDomain(topics []string, domain string) bool {
	listed := false
	for _, t := range topics {
		if t = strings.TrimSpace(t); t == "" {
			continue
		}
		listed = true
		if domainNameOf(t) == domain {
			return true
		}
	}
	return !listed
}

// commandsAllowed reports whether some connection may operate control points
// of the domain.
func commandsAllowed(connections []protocolConnection, domain string) bool {
	for _, c := range connections {
		if c.CommandsEnabled && topicAllowsDomain(c.Topics, domain) {
			return true
		}
	}
	return false
}

// iccpModel is the ICCP data model built from realtimeData, shared by every
// connection and association.
type iccpModel struct {
	dataModel   *tase2.DataModel
	ipByTag     map[string]*tase2.IndicationPoint
	cpRefByTag  map[string]tase2.ObjectRef
	tagByKey    map[string]string // "Domain/Item" -> tag
	datasetDefs []datasetDef      // all domains; each connection serves those of its topics
}

// maxDatasetItems caps a dataset's size in non-chunked mode.
const maxDatasetItems = 50000

// buildModel builds the data model: one domain per group1 (sanitized), an
// indication point per supervised or calculated tag of a standard ICCP type,
// a control point per command tag some connection may operate, and the
// domain datasets with their DSTransferSets. Tags acquired by this
// instance's own connections are skipped. Tags are processed in name order;
// a tag whose sanitized name collides with an earlier one is not exposed.
func buildModel(allTags []rtData, connections []protocolConnection, datasetChunkSize int) *iccpModel {
	m := &iccpModel{
		dataModel:  tase2.NewDataModel(),
		ipByTag:    make(map[string]*tase2.IndicationPoint),
		cpRefByTag: make(map[string]tase2.ObjectRef),
		tagByKey:   make(map[string]string),
	}
	own := ownConnections(connections)
	tags := append([]rtData(nil), allTags...)
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].Tag < tags[j].Tag })

	domainMap := make(map[string]*tase2.Domain)
	collisions := 0
	for _, tag := range tags {
		if strings.TrimSpace(tag.Group1) == "" || own[int(tag.ProtocolSourceConnectionNumber)] {
			continue
		}
		domainName := domainNameOf(tag.Group1)

		isCommand := tag.Origin == "command"
		iccpType := tase2.ICCPTypeUnknown
		if isCommand {
			// Command tags are control points only; with no connection
			// allowed to operate them they are not exposed at all.
			// commandBlocked is checked on every operate.
			if !commandsAllowed(connections, domainName) {
				continue
			}
		} else if iccpType = getICCPType(tag); iccpType == tase2.ICCPTypeUnknown {
			// String, JSON and other non-standard types are not exposed.
			continue
		}

		pointName := sanitizePointName(tag.Tag)
		key := domainName + "/" + pointName
		if prev, dup := m.tagByKey[key]; dup {
			collisions++
			LogMsg(LogLevelMin, "DataModel - Tags %q and %q both map to %s; %q is not exposed", prev, tag.Tag, key, tag.Tag)
			continue
		}
		m.tagByKey[key] = tag.Tag

		domain, ok := domainMap[domainName]
		if !ok {
			domain = m.dataModel.AddDomain(domainName)
			domainMap[domainName] = domain
		}

		if isCommand {
			deviceClass := tase2.DeviceClassDirect
			if tag.ProtocolSourceCommandUseSBO {
				deviceClass = tase2.DeviceClassSBO
			}
			cp := domain.AddControlPoint(pointName, tase2.ControlTypeCommand, deviceClass)
			cp.CurrentValue = convertToDataValue(tag)
			m.cpRefByTag[tag.Tag] = tase2.ObjectRef{Domain: domainName, Item: pointName}
			continue
		}
		ip := domain.AddDataPoint(pointName, iccpType)
		ip.UpdateValue(convertToICCPValue(tag, iccpType), nil) // quality and timestamp embedded in ICCP value
		m.ipByTag[tag.Tag] = ip
	}

	LogMsg(LogLevelNormal, "DataModel - Created %d domains, %d indication points, %d control points",
		len(domainMap), len(m.ipByTag), len(m.cpRefByTag))
	if collisions > 0 {
		LogMsg(LogLevelMin, "DataModel - %d tag(s) not exposed because of point name collisions", collisions)
	}

	// Datasets per domain, each with its DSTransferSet. When
	// datasetChunkSize > 0 and a domain has more points than the chunk size,
	// points are split across chunked datasets (SUB_A_DataSet_0001, ...) with
	// one DSTransferSet per chunk (DSTrans_0001, ...). This avoids the hidden
	// 1750-item truncation in GetNVLAttrs and gives the peer manageable,
	// fully-visible dataset chunks. Points are sorted by name for
	// deterministic membership.
	domainNames := make([]string, 0, len(domainMap))
	for name := range domainMap {
		domainNames = append(domainNames, name)
	}
	sort.Strings(domainNames)
	for _, domainName := range domainNames {
		domain := domainMap[domainName]
		pointNames := make([]string, 0, len(domain.IndicationPoints))
		for name := range domain.IndicationPoints {
			pointNames = append(pointNames, name)
		}
		if len(pointNames) == 0 {
			continue
		}
		sort.Strings(pointNames)
		dsPrefix := domainName + "_DataSet"

		if datasetChunkSize > 0 && len(pointNames) > datasetChunkSize {
			chunkIdx := 0
			for offset := 0; offset < len(pointNames); offset += datasetChunkSize {
				chunkIdx++
				end := min(offset+datasetChunkSize, len(pointNames))
				dsName := fmt.Sprintf("%s_%04d", dsPrefix, chunkIdx)
				domain.AddDSTransferSet(fmt.Sprintf("DSTrans_%04d", chunkIdx)).AttachDataSet(domainName, dsName)
				m.datasetDefs = append(m.datasetDefs, datasetDef{
					domain: domainName,
					name:   domainName + "/" + dsName,
					items:  objectRefs(domainName, pointNames[offset:end]),
				})
				LogMsg(LogLevelDetailed, "DataModel - Domain %s chunk %d: dataset '%s/%s' (%d points)",
					domainName, chunkIdx, domainName, dsName, end-offset)
			}
			LogMsg(LogLevelNormal, "DataModel - Domain %s: %d points in %d chunk(s) of %d",
				domainName, len(pointNames), chunkIdx, datasetChunkSize)
			continue
		}

		// Single-dataset mode: domain-scope DSTS with domain-scoped dataset.
		if len(pointNames) > maxDatasetItems {
			LogMsg(LogLevelMin, "DataModel - dataset %s/%s has %d items, capping to %d",
				domainName, dsPrefix, len(pointNames), maxDatasetItems)
			pointNames = pointNames[:maxDatasetItems]
		}
		domain.AddDSTransferSet("DSTrans").AttachDataSet(domainName, dsPrefix)
		m.datasetDefs = append(m.datasetDefs, datasetDef{
			domain: domainName,
			name:   domainName + "/" + dsPrefix,
			items:  objectRefs(domainName, pointNames),
		})
	}
	return m
}

func objectRefs(domain string, items []string) []tase2.ObjectRef {
	refs := make([]tase2.ObjectRef, len(items))
	for i, item := range items {
		refs[i] = tase2.ObjectRef{Domain: domain, Item: item}
	}
	return refs
}

// buildServer creates a per-connection TASE2 Server.
func buildServer(
	dataModel *tase2.DataModel,
	ce *tase2.Endpoint,
	cfg tase2.ServerConfig,
	conn protocolConnection,
	datasetDefs []datasetDef,
	cmdCollection *mongo.Collection,
) *tase2.Server {
	srv := tase2.NewServerWithConfig(dataModel, ce, cfg)

	// Add bilateral table entries if remote AP titles are configured.
	// When Topics are configured, only those domains are granted — the library
	// enforces the BLT on discovery, reads, and writes, so topics become real
	// per-connection data isolation.
	if conn.RemoteApTitle != "" {
		blt := tase2.NewBilateralTable(
			fmt.Sprintf("BLT_%d", conn.ProtocolConnectionNumber),
			conn.RemoteApTitle,
			conn.RemoteAeQualifier,
		)
		granted := 0
		for _, domainName := range dataModel.GetDomains() {
			if !topicAllowsDomain(conn.Topics, domainName) {
				continue
			}
			blt.AddDomainAccess(domainName)
			granted++
		}
		srv.AddBilateralTable(blt)
		LogMsg(LogLevelNormal, "ICCP - BLT for %s: %d domain(s) granted to %s", conn.Name, granted, conn.RemoteApTitle)
	} else if len(conn.Topics) > 0 {
		LogMsg(LogLevelMin, "ICCP - Connection %q: topics configured but no remoteApTitle set; "+
			"open mode - topics limit the predefined datasets and commands, not discovery or reads", conn.Name)
	}

	// Create the datasets of this connection's topics in its TransferSetStore
	datasets, totalItems := 0, 0
	for _, ds := range datasetDefs {
		if !topicAllowsDomain(conn.Topics, ds.domain) {
			continue
		}
		srv.TransferSets.AddDataSet(ds.name, ds.items)
		datasets++
		totalItems += len(ds.items)
	}

	// Build discovery snapshots after the model and datasets are finalised.
	// This pre-computes sorted identifier slices so handleGetNameList can
	// serve paginated discovery without rebuilding/re-sorting on every request.
	srv.BuildDiscoverySnapshots()

	LogMsg(LogLevelNormal, "ICCP - Server built for %s: %d datasets, %d total items, topics=%v",
		conn.Name, datasets, totalItems, conn.Topics)

	// Device control handlers are always registered: without one the library
	// would store the value and report success, so a connection with commands
	// disabled must refuse explicitly. The operate handler is the one the
	// library calls; the write-data handler is its fallback.
	operate := func(domain, item string, value *tase2.DataValue) tase2.HandlerResult {
		tag, ok := commandTarget(conn, "Operate", domain, item)
		if !ok {
			return tase2.ResultFailure
		}
		LogMsg(LogLevelNormal, "ICCP - Operate %s/%s = %v", domain, item, value.String())
		return forwardCommand(cmdCollection, conn, tag, value)
	}
	srv.SetOperateHandler(operate)
	srv.SetWriteDataHandler(operate)
	srv.SetSelectHandler(func(domain, item string, value *tase2.DataValue) tase2.HandlerResult {
		if _, ok := commandTarget(conn, "Select", domain, item); !ok {
			return tase2.ResultFailure
		}
		LogMsg(LogLevelNormal, "ICCP - Select %s/%s", domain, item)
		return tase2.ResultSuccess
	})

	return srv
}

// commandTarget checks that a connection may operate a control point and
// returns the JSON-SCADA command tag it maps to.
func commandTarget(conn protocolConnection, service, domain, item string) (string, bool) {
	refuse := func(reason string) (string, bool) {
		LogMsg(LogLevelMin, "ICCP - %s %s/%s refused for %s: %s", service, domain, item, conn.Name, reason)
		return "", false
	}
	if !conn.CommandsEnabled {
		return refuse("commands disabled")
	}
	if !protocolRunning() {
		return refuse("node inactive")
	}
	if !topicAllowsDomain(conn.Topics, domain) {
		return refuse("domain not in the connection topics")
	}
	tag := findTagByDomainItem(domain, item)
	if tag == "" {
		return refuse("no command tag")
	}
	rtDoc := findRtDataForTag(tag)
	if rtDoc.Origin != "command" {
		return refuse("not a command tag")
	}
	if rtDoc.CommandBlocked {
		return refuse("command blocked")
	}
	return tag, true
}

// forwardCommand inserts the command for a JSON-SCADA command tag into the
// commandsQueue collection.
func forwardCommand(cmdCollection *mongo.Collection, conn protocolConnection, tag string, value *tase2.DataValue) tase2.HandlerResult {
	rtDoc := findRtDataForTag(tag)
	if rtDoc.ID == 0 {
		LogMsg(LogLevelMin, "ICCP - Cannot find realtimeData for tag %s", tag)
		return tase2.ResultFailure
	}

	cmdValue, cmdValueStr := extractCommandValue(value)

	// Apply the tag's conversion factors to the value received from the client.
	if convValue := applyCommandConversion(cmdValue, rtDoc); convValue != cmdValue {
		LogMsg(LogLevelDetailed, "ICCP - Command %s converted %f -> %f (type=%s kconv1=%f kconv2=%f)",
			tag, cmdValue, convValue, rtDoc.Type,
			kconvToFloat64(rtDoc.Kconv1, 1), kconvToFloat64(rtDoc.Kconv2, 0))
		cmdValueStr = commandValueString(convValue, cmdValueStr)
		cmdValue = convValue
	}

	if err := insertCommand(cmdCollection, conn, tag, cmdValue, cmdValueStr, rtDoc); err != nil {
		LogMsg(LogLevelMin, "ICCP - Failed to insert command: %v", err)
		return tase2.ResultFailure
	}

	LogMsg(LogLevelNormal, "ICCP - Command forwarded: %s = %s (%f)", tag, cmdValueStr, cmdValue)
	return tase2.ResultSuccess
}

// watchRealtimeDataChanges monitors the realtimeData change stream and
// updates the shared data model; DataModel.UpdateOnlineValue keeps it current
// whether or not a client is connected and notifies every attached server.
func watchRealtimeDataChanges(
	collectionRtData *mongo.Collection,
	model *iccpModel,
	own map[int]bool,
	topicList []string,
) {
	// Flush coalesced updates at most once per second. Without this, every
	// change-stream notification would trigger an immediate DSTS report — a
	// burst of hundreds of updates would flood the clients with reports.
	go func() {
		ticker := time.NewTicker(pendingFlushInterval)
		defer ticker.Stop()
		for range ticker.C {
			flushPendingChanges(model.dataModel)
		}
	}()

	for {
		// Build change stream pipeline. Updates that carry sourceDataUpdate
		// are raw protocol driver writes: the value fields served here are
		// still the previous ones, and cs_data_processor writes the converted
		// value right after. Excluding them in this first stage (which does
		// not reference fullDocument) lets the server skip their updateLookup.
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"$or": bson.A{
					bson.M{"$and": bson.A{
						bson.M{"operationType": "update"},
						bson.M{"updateDescription.updatedFields.sourceDataUpdate": bson.M{"$exists": false}},
					}},
					bson.M{"operationType": "replace"},
				},
			}}},
		}
		if len(topicList) > 0 {
			// Restrict change stream to documents whose group1 is in the topic list
			pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{
				"fullDocument.group1": bson.M{"$in": topicList},
			}}})
		}

		opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
		cs, err := collectionRtData.Watch(context.TODO(), pipeline, opts)
		if err != nil {
			LogMsg(LogLevelMin, "ChangeStream - Error creating: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		LogMsg(LogLevelNormal, "ChangeStream - Watching realtimeData changes...")

		for cs.Next(context.TODO()) {
			var event changeEvent
			if err := cs.Decode(&event); err != nil {
				LogMsg(LogLevelMin, "ChangeStream - Decode error: %v", err)
				continue
			}

			tag := event.FullDocument

			// Skip internal system data and tags from our own connections
			if tag.ID == 0 || own[int(tag.ProtocolSourceConnectionNumber)] {
				continue
			}

			// Command tag: refresh the cached document (commandBlocked,
			// conversion factors) and the control point value.
			if ref, ok := model.cpRefByTag[tag.Tag]; ok {
				updateRtDataCache(tag)
				if err := model.dataModel.WriteValue(ref, convertToDataValue(tag)); err != nil {
					LogMsg(LogLevelDebug, "ChangeStream - Control point %s/%s: %v", ref.Domain, ref.Item, err)
				}
				continue
			}

			// Update indication point via the buffering layer: coalesce into
			// the pending map so the ticker flushes at most once per second.
			ip, ok := model.ipByTag[tag.Tag]
			if !ok {
				continue
			}
			pendingChangesMu.Lock()
			pendingChanges[ip] = convertToICCPValue(tag, ip.ICCPType)
			pendingChangesMu.Unlock()

			if currentLogLevel() >= LogLevelDebug {
				LogMsg(LogLevelDebug, "ChangeStream - Buffered %s = %v (invalid=%v)", tag.Tag, tag.Value, tag.Invalid)
			}
		}

		if err := cs.Err(); err != nil {
			LogMsg(LogLevelMin, "ChangeStream - Error: %v", err)
		}

		cs.Close(context.TODO())
		time.Sleep(5 * time.Second)
	}
}

// flushPendingChanges atomically drains the pending-changes buffer and applies
// every queued update to the data model, which notifies the attached servers.
// The drain-and-replace pattern keeps the mutex held only briefly.
func flushPendingChanges(dm *tase2.DataModel) {
	pendingChangesMu.Lock()
	old := pendingChanges
	pendingChanges = make(map[*tase2.IndicationPoint]*tase2.DataValue, len(old))
	pendingChangesMu.Unlock()

	if len(old) > 0 && currentLogLevel() >= LogLevelDetailed {
		LogMsg(LogLevelDetailed, "ChangeStream - Flushing %d pending updates to %d server(s)",
			len(old), dm.AttachedServerCount())
	}
	// Applied outside the lock: the model and server locks and MMS I/O
	// must not block the change-stream consumer.
	for ip, value := range old {
		dm.UpdateOnlineValue(ip, value, nil)
	}
}

// convertToDataValue converts a realtimeData document to a TASE2 DataValue.
func convertToDataValue(tag rtData) *tase2.DataValue {
	switch tag.Type {
	case "digital":
		return tase2.NewBooleanValue(tag.Value != 0)
	case "analog":
		// Check protocolSourceASDU for specific type
		asdu := asduToString(tag.ProtocolSourceASDU)
		switch asdu {
		case "float32", "float":
			return tase2.NewFloat32Value(float32(tag.Value))
		case "int16":
			return tase2.NewIntegerValue(int64(int16(tag.Value)))
		case "uint16":
			return tase2.NewUnsignedValue(uint64(uint16(tag.Value)))
		case "int32":
			return tase2.NewIntegerValue(int64(int32(tag.Value)))
		case "uint32":
			return tase2.NewUnsignedValue(uint64(uint32(tag.Value)))
		case "int64":
			return tase2.NewIntegerValue(int64(tag.Value))
		case "boolean":
			return tase2.NewBooleanValue(tag.Value != 0)
		default:
			return tase2.NewFloat32Value(float32(tag.Value))
		}
	case "string":
		if tag.ValueString != "" {
			return tase2.NewVisibleStringValue(tag.ValueString)
		}
		return tase2.NewVisibleStringValue(fmt.Sprintf("%v", tag.Value))
	default:
		return tase2.NewFloat32Value(float32(tag.Value))
	}
}

// tagQuality maps the realtimeData quality flags to an ICCP quality.
// Validity: invalid > transient (suspect) > frozen (held) > valid. Source:
// substituted or manual tags are entered, calculated tags calculated, the
// rest telemetered.
func tagQuality(tag rtData) *tase2.Quality {
	q := &tase2.Quality{Validity: tase2.QualityValid, Source: tase2.SourceTelemetered}
	switch {
	case tag.Invalid:
		q.Validity = tase2.QualityInvalid
	case tag.Transient:
		q.Validity = tase2.QualitySuspect
	case tag.Frozen:
		q.Validity = tase2.QualityHeld
	}
	switch {
	case tag.Substituted || tag.Origin == "manual":
		q.Source = tase2.SourceEntered
	case tag.Origin == "calculated":
		q.Source = tase2.SourceCalculated
	}
	return q
}

// getICCPType maps a realtimeData tag type to an ICCP data type with quality
// and a millisecond-resolution time stamp (IEC 60870-6-802 Ed.2
// QTimeTagExtended). Returns ICCPTypeUnknown for types that don't map to a
// standard ICCP type (e.g. strings, JSON).
func getICCPType(tag rtData) tase2.ICCPType {
	switch tag.Type {
	case "digital":
		return tase2.ICCPTypeStateQTimeTagExtended
	case "analog":
		// Integer-typed analogs map to Discrete to avoid float32 precision
		// loss on large values.
		switch asduToString(tag.ProtocolSourceASDU) {
		case "int16", "uint16", "int32", "uint32", "int64", "uint64", "integer":
			return tase2.ICCPTypeDiscreteQTimeTagExtended
		}
		return tase2.ICCPTypeRealQTimeTagExtended
	default:
		return tase2.ICCPTypeUnknown
	}
}

// convertToICCPValue converts a realtimeData document to an ICCP-typed DataValue
// using the appropriate constructor for the given ICCP type. The returned value
// embeds quality and timestamp per the ICCP data type specification.
func convertToICCPValue(tag rtData, iccpType tase2.ICCPType) *tase2.DataValue {
	q := tagQuality(tag)
	ts := time.Now()
	if tag.TimeTagAtSource != nil && tag.TimeTagAtSourceOk {
		ts = *tag.TimeTagAtSource
	}
	state := tase2.StateOff
	if tag.Value != 0 {
		state = tase2.StateOn
	}

	switch iccpType {
	case tase2.ICCPTypeStateQTimeTagExtended:
		return tase2.NewStateQTimeTagExtended(state, q, ts)
	case tase2.ICCPTypeRealQTimeTagExtended:
		return tase2.NewRealQTimeTagExtended(float32(tag.Value), q, ts)
	case tase2.ICCPTypeDiscreteQTimeTagExtended:
		return tase2.NewDiscreteQTimeTagExtended(int64(tag.Value), q, ts)
	// Second-resolution (GMTBasedS) variants, for points declared with them.
	case tase2.ICCPTypeStateQTimeTag:
		return tase2.NewStateQTimeTag(state, q, tase2.TimeStampFrom(ts))
	case tase2.ICCPTypeRealQTimeTag:
		return tase2.NewRealQTimeTag(float32(tag.Value), q, tase2.TimeStampFrom(ts))
	case tase2.ICCPTypeDiscreteQTimeTag:
		return tase2.NewDiscreteQTimeTag(int64(tag.Value), q, tase2.TimeStampFrom(ts))
	default:
		return convertToDataValue(tag)
	}
}

// sanitizePointName converts any non-alphanumeric character to underscore,
// ensures the result begins with a letter (a-z, A-Z) and truncates it to the
// 32 characters of an MMS identifier.
func sanitizePointName(name string) string {
	if name == "" {
		return "X_unnamed_"
	}

	// Replace any non-alphanumeric character with underscore
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	result := b.String()

	// Ensure name begins with a letter
	if (result[0] < 'A' || result[0] > 'Z') && (result[0] < 'a' || result[0] > 'z') {
		result = "X" + result
	}

	// Truncate to 32 chars
	if len(result) > 32 {
		result = result[:32]
	}
	return result
}

// Change-stream buffering: the latest value of each indication point is kept
// in pendingChanges and flushed every pendingFlushInterval by the ticker
// goroutine started in watchRealtimeDataChanges.
var (
	pendingChangesMu sync.Mutex
	pendingChanges   = make(map[*tase2.IndicationPoint]*tase2.DataValue)
)

const pendingFlushInterval = 1 * time.Second

// Cache for command routing: the realtimeData documents of the exposed tags
// and the "Domain/Item" -> tag lookup, populated at startup; command tags are
// refreshed from the change stream.
var (
	rtDataByTag     = make(map[string]rtData)
	tagByDomainItem = make(map[string]string)
	rtDataByTagMu   sync.Mutex
)

// findTagByDomainItem finds the original JSON-SCADA tag given a domain and item name.
func findTagByDomainItem(domain, item string) string {
	rtDataByTagMu.Lock()
	defer rtDataByTagMu.Unlock()
	return tagByDomainItem[domain+"/"+item]
}

// findRtDataForTag looks up the realtimeData document for a given tag.
func findRtDataForTag(tag string) rtData {
	rtDataByTagMu.Lock()
	defer rtDataByTagMu.Unlock()
	return rtDataByTag[tag]
}

// populateRtDataCache stores the documents of the exposed tags (tagByKey,
// from buildModel) for command routing.
func populateRtDataCache(tags []rtData, tagByKey map[string]string) {
	exposed := make(map[string]bool, len(tagByKey))
	for _, t := range tagByKey {
		exposed[t] = true
	}
	rtDataByTagMu.Lock()
	defer rtDataByTagMu.Unlock()
	rtDataByTag = make(map[string]rtData, len(tagByKey))
	tagByDomainItem = make(map[string]string, len(tagByKey))
	for k, t := range tagByKey {
		tagByDomainItem[k] = t
	}
	for _, t := range tags {
		if exposed[t.Tag] {
			rtDataByTag[t.Tag] = t
		}
	}
}

// updateRtDataCache replaces the cached document of an exposed tag.
func updateRtDataCache(tag rtData) {
	rtDataByTagMu.Lock()
	defer rtDataByTagMu.Unlock()
	if _, ok := rtDataByTag[tag.Tag]; ok {
		rtDataByTag[tag.Tag] = tag
	}
}

// extractCommandValue extracts a numeric value and string from a TASE2 DataValue.
func extractCommandValue(dv *tase2.DataValue) (float64, string) {
	if dv == nil {
		return 0, ""
	}
	switch {
	case dv.BoolVal != nil:
		if *dv.BoolVal {
			return 1, "true"
		}
		return 0, "false"
	case dv.IntVal != nil:
		return float64(*dv.IntVal), fmt.Sprintf("%d", *dv.IntVal)
	case dv.UnsignedVal != nil:
		return float64(*dv.UnsignedVal), fmt.Sprintf("%d", *dv.UnsignedVal)
	case dv.FloatVal != nil:
		return *dv.FloatVal, fmt.Sprintf("%f", *dv.FloatVal)
	case dv.VisibleStringVal != "":
		v, _ := strconv.ParseFloat(dv.VisibleStringVal, 64)
		return v, dv.VisibleStringVal
	default:
		return 0, dv.String()
	}
}

// applyCommandConversion adjusts a command value received from an ICCP client
// using the destination tag's type and conversion factors from realtimeData,
// mirroring what the IEC 60870-5-104 server driver does when routing commands:
//
//	digital, kconv1 == -1: forced to 1/0 and inverted
//	digital, otherwise:    forced to 1/0
//	analog:                value*kconv1 + kconv2
//
// Any other type is forwarded unchanged.
func applyCommandConversion(value float64, rtDoc rtData) float64 {
	kconv1 := kconvToFloat64(rtDoc.Kconv1, 1)

	switch rtDoc.Type {
	case "digital":
		v := 0.0
		if value != 0 {
			v = 1
		}
		if kconv1 == -1 { // invert digital for kconv1 -1
			v = 1 - v
		}
		return v
	case "analog":
		return value*kconv1 + kconvToFloat64(rtDoc.Kconv2, 0)
	default:
		return value
	}
}

// commandValueString renders a converted command value, keeping the boolean or
// numeric flavor of the value originally received from the client.
func commandValueString(value float64, origStr string) string {
	switch origStr {
	case "true", "false":
		if value != 0 {
			return "true"
		}
		return "false"
	}
	if value == math.Trunc(value) && math.Abs(value) < 1e15 {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
