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

// datasetDef holds a dataset name and its member ObjectRefs.
type datasetDef struct {
	name  string
	items []tase2.ObjectRef
}

// serverEntry tracks a live per-connection TASE2 server in the registry.
type serverEntry struct {
	server     *tase2.Server
	connection protocolConnection
	endpoint   *tase2.Endpoint
}

// serverRegistry tracks all live per-connection servers.
type serverRegistry struct {
	mu      sync.Mutex
	entries map[*tase2.Server]*serverEntry
}

func newServerRegistry() *serverRegistry {
	return &serverRegistry{entries: make(map[*tase2.Server]*serverEntry)}
}

func (r *serverRegistry) add(srv *tase2.Server, ep *tase2.Endpoint, conn protocolConnection) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[srv] = &serverEntry{server: srv, endpoint: ep, connection: conn}
	return len(r.entries)
}

func (r *serverRegistry) remove(srv *tase2.Server) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entries[srv]; ok {
		e.endpoint.Transport.Close()
	}
	delete(r.entries, srv)
	return len(r.entries)
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
	currentLogLevel = instLogLevel
	configureTASE2Logging(currentLogLevel)
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
	LogMsg(LogLevelMin, "Config - Log level: %d", currentLogLevel)
	LogMsg(LogLevelMin, "Config - GetNameList max identifiers: %d", getNameListMaxPerResponse)
	LogMsg(LogLevelMin, "Config - GetNameList max estimated bytes: %d", getNameListMaxBytes)
	LogMsg(LogLevelMin, "Config - Max TPDU size parameter: %d", maxTPDUSizeParam)
	if datasetChunkSize > 0 {
		LogMsg(LogLevelMin, "Config - Dataset chunk size: %d", datasetChunkSize)
	} else {
		LogMsg(LogLevelMin, "Config - Dataset chunking: disabled (single dataset per domain)")
	}

	var clientMongo *mongo.Client
	var collectionRtData, collectionInstances, collectionConnections, collectionCommands *mongo.Collection

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
			collectionRtData = db.Collection("realtimeData")
			collectionInstances = db.Collection("protocolDriverInstances")
			collectionConnections = db.Collection("protocolConnections")
			collectionCommands = db.Collection("commandsQueue")

			// Verify instance config exists
			_, err = getInstanceConfig(collectionInstances, instanceNumber)
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

			// Build the shared data model from realtimeData
			dataModel := tase2.NewDataModel()
			domainMap := make(map[string]*tase2.Domain)
			ipByTag := make(map[string]*tase2.IndicationPoint)
			cpByTag := make(map[string]*tase2.ControlPoint)
			var allTags []rtData

			// Build filter for topics across all connections
			connNumbers := make(map[int]bool)
			allTopics := make(map[string]bool) // union of Topics from all connections
			hasTopics := false
			for _, conn := range connections {
				connNumbers[conn.ProtocolConnectionNumber] = true
				for _, t := range conn.Topics {
					t := strings.TrimSpace(t)
					if t != "" {
						allTopics[t] = true
						hasTopics = true
					}
				}
			}

			// Build unique topic list for MongoDB $in filter
			var topicList []string
			if hasTopics {
				topicList = make([]string, 0, len(allTopics))
				for t := range allTopics {
					topicList = append(topicList, t)
				}
			}

			// Query realtimeData for tags not owned by these connections
			filter := bson.D{
				{Key: "_id", Value: bson.M{"$gt": 0}}, // exclude internal system data
			}
			if hasTopics {
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
			}

			cursor, err := collectionRtData.Find(context.TODO(), filter, options.Find().SetProjection(projection).SetSort(bson.D{{Key: "protocolSourceConnectionNumber", Value: 1}}))
			if err != nil {
				LogMsg(LogLevelMin, "MongoDB - Error querying realtimeData: %v", err)
				clientMongo.Disconnect(context.TODO())
				clientMongo = nil
				time.Sleep(5 * time.Second)
				continue
			}

			if err := cursor.All(context.TODO(), &allTags); err != nil {
				LogMsg(LogLevelMin, "MongoDB - Error reading realtimeData: %v", err)
				cursor.Close(context.TODO())
				clientMongo.Disconnect(context.TODO())
				clientMongo = nil
				time.Sleep(5 * time.Second)
				continue
			}
			cursor.Close(context.TODO())

			LogMsg(LogLevelNormal, "DataModel - Loaded %d tags from realtimeData", len(allTags))

			// Build domains and points
			for _, tag := range allTags {
				// Determine group1 (domain name)
				group1 := strings.TrimSpace(tag.Group1)
				if group1 == "" {
					// Skip tags with no group1
					continue
				}

				// Check if tag belongs to any of our own connections (skip those)
				if connNumbers[int(tag.ProtocolSourceConnectionNumber)] {
					continue
				}

				// Get or create domain
				domain, ok := domainMap[group1]
				if !ok {
					domain = dataModel.AddDomain(group1)
					domainMap[group1] = domain
				}

				// Use the tag field as the ICCP point name
				pointName := sanitizePointName(tag.Tag)

				// If tag is a command, add as control point
				if tag.Origin == "command" && !tag.CommandBlocked {
					// Check if any connection has commands enabled
					hasCmdEnabled := false
					for _, conn := range connections {
						if conn.CommandsEnabled {
							// Check topic filter
							if len(conn.Topics) > 0 && !contains(conn.Topics, group1) {
								continue
							}
							hasCmdEnabled = true
							break
						}
					}
					if hasCmdEnabled {
						cp := domain.AddControlPoint(pointName, tase2.ControlTypeCommand, tase2.DeviceClassSBO)
						cp.CurrentValue = convertToDataValue(tag)
						cpByTag[tag.Tag] = cp
						continue
					}
				}

				// Add as indication point only when the tag type maps to a standard
				// ICCP data type (digital→StateQTimeTag, analog→RealQTimeTag).
				// String, JSON, and other non-standard types are not exposed via ICCP.
				iccpType := getICCPType(tag)
				if iccpType != tase2.ICCPTypeUnknown {
					ip := domain.AddDataPoint(pointName, iccpType)
					val := convertToICCPValue(tag, iccpType)
					ip.UpdateValue(val, nil) // quality and timestamp embedded in ICCP value
					ipByTag[tag.Tag] = ip
				}
			}

			LogMsg(LogLevelNormal, "DataModel - Created %d domains, %d indication points, %d control points",
				len(domainMap), len(ipByTag), len(cpByTag))

			// Create datasets per domain and attach DSTransferSets.
			// When datasetChunkSize > 0 and a domain has more points than the
			// chunk size, points are split across multiple chunked datasets
			// (e.g. SUB_A_DataSet_0001, SUB_A_DataSet_0002, ...) with one
			// DSTransferSet per chunk (e.g. DSTrans_0001, DSTrans_0002, ...).
			// This avoids the hidden 1750-item truncation in GetNVLAttrs and
			// gives the peer manageable, fully-visible dataset chunks.
			for domainName, domain := range domainMap {
				if len(domain.IndicationPoints) == 0 && len(domain.ControlPoints) == 0 {
					continue
				}
				dsPrefix := sanitizePointName(domainName) + "_DataSet"

				if datasetChunkSize > 0 && len(domain.IndicationPoints) > datasetChunkSize {
					// Chunked mode: create one dataset and one DSTransferSet per chunk.
					// Dataset names: SUB_A_DataSet_0001, SUB_A_DataSet_0002, ...
					// DSTS names:    DSTrans_0001, DSTrans_0002, ...
					// Points are sorted by name for deterministic chunk membership.
					pointNames := make([]string, 0, len(domain.IndicationPoints))
					for name := range domain.IndicationPoints {
						pointNames = append(pointNames, name)
					}
					sort.Strings(pointNames)

					chunkIdx := 0
					for offset := 0; offset < len(pointNames); offset += datasetChunkSize {
						chunkIdx++
						end := offset + datasetChunkSize
						if end > len(pointNames) {
							end = len(pointNames)
						}
						chunkPointNames := pointNames[offset:end]

						dsName := fmt.Sprintf("%s_%04d", dsPrefix, chunkIdx)
						dstsName := fmt.Sprintf("DSTrans_%04d", chunkIdx)

						dsts := domain.AddDSTransferSet(dstsName)
						dsts.AttachDataSet(domainName, dsName)

						LogMsg(LogLevelDetailed, "DataModel - Domain %s chunk %d: dataset '%s' (%d points)",
							domainName, chunkIdx, domainName+"/"+dsName, len(chunkPointNames))
					}
				} else {
					// Single-dataset mode: domain-scope DSTS with domain-scoped dataset.
					dsts := domain.AddDSTransferSet("DSTrans")
					dsts.AttachDataSet(domainName, dsPrefix)
				}
			}

			// Populate the rtData cache for command processing
			populateRtDataCache(allTags)

			// Store dataset definitions for per-connection creation.
			// When chunking is enabled, split large domains into multiple
			// datasets of at most datasetChunkSize items each.
			const maxDatasetItems = 50000 // cap dataset size (non-chunked mode)
			var datasetDefs []datasetDef
			for domainName, domain := range domainMap {
				if datasetChunkSize > 0 && len(domain.IndicationPoints) > datasetChunkSize {
					// Chunked mode: sort points, split into chunks
					pointNames := make([]string, 0, len(domain.IndicationPoints))
					for name := range domain.IndicationPoints {
						pointNames = append(pointNames, name)
					}
					sort.Strings(pointNames)

					dsPrefix := sanitizePointName(domainName) + "_DataSet"
					chunkIdx := 0
					for offset := 0; offset < len(pointNames); offset += datasetChunkSize {
						chunkIdx++
						end := offset + datasetChunkSize
						if end > len(pointNames) {
							end = len(pointNames)
						}
						dsName := fmt.Sprintf("%s_%04d", dsPrefix, chunkIdx)
						var dsItems []tase2.ObjectRef
						for _, pn := range pointNames[offset:end] {
							dsItems = append(dsItems, tase2.ObjectRef{Domain: domainName, Item: pn})
						}
						datasetDefs = append(datasetDefs, datasetDef{name: domainName + "/" + dsName, items: dsItems})
					}
					LogMsg(LogLevelNormal, "DataModel - Domain %s: %d points in %d chunk(s) of %d",
						domainName, len(pointNames), chunkIdx, datasetChunkSize)
				} else {
					// Single-dataset mode: store dataset under domain-scoped key.
					dsName := domainName + "/" + sanitizePointName(domainName) + "_DataSet"
					var dsItems []tase2.ObjectRef
					for name := range domain.IndicationPoints {
						dsItems = append(dsItems, tase2.ObjectRef{Domain: domainName, Item: name})
					}
					if len(dsItems) > maxDatasetItems {
						LogMsg(LogLevelMin, "DataModel - dataset %s has %d items, capping to %d",
							dsName, len(dsItems), maxDatasetItems)
						dsItems = dsItems[:maxDatasetItems]
					}
					sort.Slice(dsItems, func(i, j int) bool {
						return dsItems[i].Item < dsItems[j].Item
					})
					if len(dsItems) > 0 {
						datasetDefs = append(datasetDefs, datasetDef{name: dsName, items: dsItems})
					}
				}
			}

			// Start servers for each connection
			registry := newServerRegistry()

			for _, conn := range connections {
				conn := conn // capture for closure
				port := 102  // default ICCP port
				if conn.IPAddressLocalBind != "" {
					parts := strings.Split(conn.IPAddressLocalBind, ":")
					if len(parts) > 1 {
						if p, err := strconv.Atoi(parts[1]); err == nil {
							port = p
						}
					}
				}

				localAPTitle := conn.LocalApTitle
				localAEQual := conn.LocalAeQualifier
				if localAPTitle == "" {
					localAPTitle = fmt.Sprintf("1.1.999.%d", conn.ProtocolConnectionNumber)
				}
				if localAEQual == 0 {
					localAEQual = 12
				}

				cfg := tase2.ServerConfig{
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
					AuthenticationPassword:    conn.AuthenticationPassword,
					GetNameListMaxPerResponse: getNameListMaxPerResponse,
					GetNameListMaxBytes:       getNameListMaxBytes,
					MaxNVLAttrsItems:          maxNVLAttrsItems,
				}

				endpoint := tase2.NewEndpoint(tase2.EndpointPassive)
				endpoint.SetLocalAPTitle(localAPTitle, localAEQual)
				endpoint.SetMaxTPDUSizeParam(maxTPDUSizeParam)

				if err := endpoint.Listen(port); err != nil {
					LogMsg(LogLevelMin, "ICCP - Cannot listen on port %d: %v", port, err)
					continue
				}

				LogMsg(LogLevelMin, "ICCP - Connection %q listening on port %d (AP: %s, AE: %d)",
					conn.Name, port, localAPTitle, localAEQual)

				// Accept loop for each connection
				go func(ep *tase2.Endpoint, conn protocolConnection, srvCfg tase2.ServerConfig) {
					for {
						ce, err := ep.AcceptEndpoint()
						if err != nil {
							if errors.Is(err, net.ErrClosed) {
								LogMsg(LogLevelNormal, "ICCP - Listener closed for %s", conn.Name)
								return
							}
							LogMsg(LogLevelMin, "ICCP - Accept error for %s: %v", conn.Name, err)
							continue
						}

						// IP filtering is done at the ICCP bilateral-table level;
						// IP-based access control can be added in a future version.

						LogMsg(LogLevelNormal, "ICCP - Client connected to %s", conn.Name)

						go serveClient(ce, dataModel, srvCfg, conn, registry, datasetDefs, collectionCommands)
					}
				}(endpoint, conn, cfg)
			}

			// Start the MongoDB change stream watcher
			go watchRealtimeDataChanges(clientMongo, collectionRtData, registry, ipByTag, cpByTag, connNumbers, topicList)

			// Start redundancy keepalive loop
			go func() {
				instance, err := getInstanceConfig(collectionInstances, instanceNumber)
				if err != nil {
					LogMsg(LogLevelMin, "Redundancy - Cannot load instance config: %v", err)
					return
				}
				for {
					processRedundancy(collectionInstances, instance.ID, cfg)
					time.Sleep(5 * time.Second)
				}
			}()

			LogMsg(LogLevelMin, "ICCP - All servers started. Waiting for connections...")
		}

		// Keep-alive: check MongoDB connection
		time.Sleep(5 * time.Second)
		err := clientMongo.Ping(context.TODO(), nil)
		if err != nil {
			LogMsg(LogLevelMin, "MongoDB - Connection lost: %v", err)
			clientMongo.Disconnect(context.TODO())
			clientMongo = nil
		}
	}
}

func configureTASE2Logging(level int) {
	switch {
	case level >= LogLevelDebug:
		tase2.SetLogLevel(tase2.LogLevelDebug)
	case level >= LogLevelDetailed:
		tase2.SetLogLevel(tase2.LogLevelInfo)
	default:
		tase2.SetLogLevel(tase2.LogLevelError)
	}
}

// serveClient builds and runs a TASE2 Server for one accepted client connection.
func serveClient(
	ce *tase2.Endpoint,
	dataModel *tase2.DataModel,
	cfg tase2.ServerConfig,
	conn protocolConnection,
	registry *serverRegistry,
	datasetDefs []datasetDef,
	cmdCollection *mongo.Collection,
) {
	// Build per-connection server
	srv := buildServer(dataModel, ce, cfg, conn, datasetDefs, cmdCollection)

	n := registry.add(srv, ce, conn)
	LogMsg(LogLevelNormal, "ICCP - Client associated! %s (%d active client(s))",
		conn.Name, n)

	if err := srv.Start(); err != nil {
		LogMsg(LogLevelNormal, "ICCP - Server error for %s: %v", conn.Name, err)
	}

	n = registry.remove(srv)
	LogMsg(LogLevelNormal, "ICCP - Client disconnected from %s (%d active client(s))",
		conn.Name, n)
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

	// Add bilateral table entries if remote AP titles are configured
	if conn.RemoteApTitle != "" {
		blt := tase2.NewBilateralTable(
			fmt.Sprintf("BLT_%d", conn.ProtocolConnectionNumber),
			conn.RemoteApTitle,
			conn.RemoteAeQualifier,
		)
		// Grant access to all domains
		for _, domainName := range dataModel.GetDomains() {
			blt.AddDomainAccess(domainName)
		}
		srv.AddBilateralTable(blt)
	}

	// Create datasets in this connection's TransferSetStore
	for _, ds := range datasetDefs {
		srv.TransferSets.AddDataSet(ds.name, ds.items)
	}

	// Build discovery snapshots after the model and datasets are finalised.
	// This pre-computes sorted identifier slices so handleGetNameList can
	// serve paginated discovery without rebuilding/re-sorting on every request.
	srv.BuildDiscoverySnapshots()

	totalItems := 0
	for _, ds := range datasetDefs {
		totalItems += len(ds.items)
	}
	LogMsg(LogLevelNormal, "ICCP - Server built for %s: %d datasets, %d total items, topics=%v",
		conn.Name, len(datasetDefs), totalItems, conn.Topics)

	// Register device control handlers for command forwarding
	if conn.CommandsEnabled {
		srv.SetOperateHandler(func(domain, item string, value *tase2.DataValue) tase2.HandlerResult {
			LogMsg(LogLevelNormal, "ICCP - Operate %s/%s = %v", domain, item, value.String())
			return tase2.ResultSuccess
		})

		srv.SetSelectHandler(func(domain, item string, value *tase2.DataValue) tase2.HandlerResult {
			LogMsg(LogLevelNormal, "ICCP - Select %s/%s = %v", domain, item, value.String())
			return tase2.ResultSuccess
		})

		srv.SetWriteDataHandler(func(domain, item string, value *tase2.DataValue) tase2.HandlerResult {
			LogMsg(LogLevelNormal, "ICCP - Write %s/%s = %v", domain, item, value.String())

			// Find the corresponding command tag and forward to commandsQueue
			// We need to find the original tag from realtimeData by matching domain+pointName
			tag := findTagByDomainItem(domain, item)
			if tag == "" {
				LogMsg(LogLevelMin, "ICCP - Cannot find tag for %s/%s", domain, item)
				return tase2.ResultFailure
			}

			// Convert DataValue to command value
			cmdValue, cmdValueStr := extractCommandValue(value)

			// Create a minimal rtData for the insert
			rtDoc := findRtDataForTag(tag)
			if rtDoc.ID == 0 {
				LogMsg(LogLevelMin, "ICCP - Cannot find realtimeData for tag %s", tag)
				return tase2.ResultFailure
			}

			err := insertCommand(cmdCollection, conn, tag, cmdValue, cmdValueStr, rtDoc)
			if err != nil {
				LogMsg(LogLevelMin, "ICCP - Failed to insert command: %v", err)
				return tase2.ResultFailure
			}

			LogMsg(LogLevelNormal, "ICCP - Command forwarded: %s = %s (%f)",
				tag, cmdValueStr, cmdValue)
			return tase2.ResultSuccess
		})
	}

	return srv
}

// watchRealtimeDataChanges monitors MongoDB change stream and pushes updates to all servers.
func watchRealtimeDataChanges(
	client *mongo.Client,
	collectionRtData *mongo.Collection,
	registry *serverRegistry,
	ipByTag map[string]*tase2.IndicationPoint,
	cpByTag map[string]*tase2.ControlPoint,
	connNumbers map[int]bool,
	topicList []string,
) {
	// Start a ticker goroutine that flushes coalesced updates at most once per
	// second. Without this, every change-stream notification would trigger an
	// immediate DSTS report — a burst of hundreds of updates would flood the
	// client with reports.
	stopFlushCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pendingFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopFlushCh:
				// Final flush before exit: drain remaining buffered changes.
				flushPendingChanges()
				return
			case <-ticker.C:
				flushPendingChanges()
			}
		}
	}()

	for {
		// Build change stream pipeline
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"$or": bson.A{
					bson.M{"operationType": "update"},
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

			// Skip internal system data
			if tag.ID == 0 {
				continue
			}

			// Skip tags from our own connections
			if connNumbers[int(tag.ProtocolSourceConnectionNumber)] {
				continue
			}

			// Check if this tag is a command (control point update)
			if tag.Origin == "command" {
				if _, ok := cpByTag[tag.Tag]; ok {
					newVal := convertToDataValue(tag)
					servers := registry.snapshot()
					for _, entry := range servers {
						entry.server.DataModel.WriteValue(
							tase2.ObjectRef{Domain: tag.Group1, Item: sanitizePointName(tag.Tag)},
							newVal,
						)
					}
				}
				continue
			}

			// Update indication point via the buffering layer: coalesce into
			// the pending map so the ticker flushes at most once per second.
			ip, ok := ipByTag[tag.Tag]
			if !ok {
				continue
			}

			var newVal *tase2.DataValue
			var newQual *tase2.Quality
			if ip.ICCPType != tase2.ICCPTypeUnknown {
				newVal = convertToICCPValue(tag, ip.ICCPType)
			} else {
				newVal, newQual = convertToDataValueWithQuality(tag)
			}

			servers := registry.snapshot()
			pendingChangesMu.Lock()
			for _, entry := range servers {
				// Check topic filter for this connection
				if len(entry.connection.Topics) > 0 {
					if !contains(entry.connection.Topics, tag.Group1) {
						continue
					}
				}
				srv := entry.server
				srvMap := pendingChangesMap[srv]
				if srvMap == nil {
					srvMap = make(map[*tase2.IndicationPoint]pendingPointUpdate)
					pendingChangesMap[srv] = srvMap
				}
				// Keep only the latest value for each point — overwrite any
				// earlier change that hasn't been flushed yet.
				srvMap[ip] = pendingPointUpdate{ip: ip, value: newVal, quality: newQual}
			}
			pendingChangesMu.Unlock()

			if currentLogLevel >= LogLevelDebug {
				LogMsg(LogLevelDebug, "ChangeStream - Buffered %s = %v (invalid=%v) for %d server(s)",
					tag.Tag, tag.Value, tag.Invalid, len(servers))
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
// every queued update via server.UpdateOnlineValue. The drain-and-replace
// pattern keeps the mutex held only briefly (O(map size) memory, not O(n) I/O).
func flushPendingChanges() {
	// Atomically extract the pending map and replace it with a fresh empty one.
	pendingChangesMu.Lock()
	old := pendingChangesMap
	pendingChangesMap = make(map[*tase2.Server]map[*tase2.IndicationPoint]pendingPointUpdate, len(old))
	pendingChangesMu.Unlock()

	totalUpdates := 0
	for _, srvMap := range old {
		totalUpdates += len(srvMap)
	}
	if totalUpdates > 0 && currentLogLevel >= LogLevelDetailed {
		LogMsg(LogLevelDetailed, "ChangeStream - Flushing %d pending updates across %d servers",
			totalUpdates, len(old))
	}

	// Apply all updates outside the lock so UpdateOnlineValue (which may
	// acquire server-level locks and perform MMS I/O) doesn't block the
	// change-stream consumer.
	for srv, srvMap := range old {
		if currentLogLevel >= LogLevelDebug && len(srvMap) > 0 {
			LogMsg(LogLevelDebug, "ChangeStream - Flushing %d updates to server %p", len(srvMap), srv)
		}
		for _, upd := range srvMap {
			srv.UpdateOnlineValue(upd.ip, upd.value, upd.quality)
		}
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

// convertToDataValueWithQuality converts a realtimeData document to a TASE2 DataValue and Quality.
func convertToDataValueWithQuality(tag rtData) (*tase2.DataValue, *tase2.Quality) {
	val := convertToDataValue(tag)
	var qual *tase2.Quality
	if tag.Invalid {
		qual = &tase2.Quality{Validity: "invalid", Source: "process"}
	} else {
		qual = &tase2.Quality{Validity: "good", Source: "process"}
	}
	return val, qual
}

// getICCPType maps a realtimeData tag type to an ICCP data type with quality
// and timestamp. Returns ICCPTypeUnknown for types that don't map to a
// standard ICCP type (e.g. strings, JSON).
func getICCPType(tag rtData) tase2.ICCPType {
	switch tag.Type {
	case "digital":
		return tase2.ICCPTypeStateQTimeTag
	case "analog":
		return tase2.ICCPTypeRealQTimeTag
	default:
		return tase2.ICCPTypeUnknown
	}
}

// convertToICCPValue converts a realtimeData document to an ICCP-typed DataValue
// using the appropriate constructor for the given ICCP type. The returned value
// embeds quality and timestamp per the ICCP data type specification.
func convertToICCPValue(tag rtData, iccpType tase2.ICCPType) *tase2.DataValue {
	q := &tase2.Quality{Validity: "good", Source: "process"}
	if tag.Invalid {
		q = &tase2.Quality{Validity: "invalid", Source: "process"}
	}
	tod := tase2.TimeTagNow()
	if tag.TimeTagAtSource != nil && tag.TimeTagAtSourceOk {
		tod = tase2.TimeTagFrom(*tag.TimeTagAtSource)
	}

	switch iccpType {
	case tase2.ICCPTypeStateQTimeTag:
		state := tase2.StateOff
		if tag.Value != 0 {
			state = tase2.StateOn
		}
		return tase2.NewStateQTimeTag(state, q, tod)
	case tase2.ICCPTypeRealQTimeTag:
		return tase2.NewRealQTimeTag(float32(tag.Value), q, tod)
	case tase2.ICCPTypeDiscreteQTimeTag:
		return tase2.NewDiscreteQTimeTag(int64(tag.Value), q, tod)
	default:
		val, _ := convertToDataValueWithQuality(tag)
		return val
	}
}

// sanitizePointName converts any non-alphanumeric character to underscore
// and ensures the result begins with a letter (a-z, A-Z).
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

// pendingPointUpdate holds a buffered indication-point update that will be
// flushed to the server on the next tick (coalescing rapid-fire change stream
// notifications into at-most-once-per-second DSTS reports).
type pendingPointUpdate struct {
	ip      *tase2.IndicationPoint
	value   *tase2.DataValue
	quality *tase2.Quality
}

// Change-stream buffering: updates are collected per-server into
// pendingChangesMap and flushed every pendingFlushInterval by the ticker
// goroutine started in watchRealtimeDataChanges.
var (
	pendingChangesMu  sync.Mutex
	pendingChangesMap = make(map[*tase2.Server]map[*tase2.IndicationPoint]pendingPointUpdate)
)

const pendingFlushInterval = 1 * time.Second

// A global cache for tag-to-realtimeData lookup, populated at startup.
var rtDataByTag = make(map[string]rtData)
var tagByDomainItem = make(map[string]string) // "domain/item" -> tag
var rtDataByTagMu sync.Mutex

// findTagByDomainItem finds the original JSON-SCADA tag given a domain and item name.
// This is used during command processing to find the source tag for command routing.
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

// populateRtDataCache stores realtimeData documents for later lookup.
func populateRtDataCache(tags []rtData) {
	rtDataByTagMu.Lock()
	defer rtDataByTagMu.Unlock()
	rtDataByTag = make(map[string]rtData, len(tags))
	tagByDomainItem = make(map[string]string, len(tags))
	for _, t := range tags {
		rtDataByTag[t.Tag] = t
		pointName := sanitizePointName(t.Tag)
		if t.Group1 != "" && pointName != "" {
			tagByDomainItem[t.Group1+"/"+pointName] = t.Tag
		}
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
