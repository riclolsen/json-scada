/*
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

// Configuration: json-scada.json plus the JS_CSDATAPROC_* environment
// variables. Argument order is the same as the Node.js version
// (instance, log level, config file), so service definitions transfer as is.

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Config holds everything the process needs at runtime.
type Config struct {
	// From json-scada.json
	NodeName                 string `json:"nodeName"`
	MongoConnectionString    string `json:"mongoConnectionString"`
	MongoDatabaseName        string `json:"mongoDatabaseName"`
	TLSCaPemFile             string `json:"tlsCaPemFile"`
	TLSClientPemFile         string `json:"tlsClientPemFile"`
	TLSClientKeyPassword     string `json:"tlsClientKeyPassword"`
	TLSAllowInvalidHostnames bool   `json:"tlsAllowInvalidHostnames"`
	TLSAllowChainErrors      bool   `json:"tlsAllowChainErrors"`
	TLSInsecure              bool   `json:"tlsInsecure"`

	// Runtime
	Instance    int    `json:"-"`
	LogLevel    int    `json:"-"`
	SQLFilesDir string `json:"-"`

	// Change stream divide expression (JS_CSDATAPROC_DIVIDE_EXP), an extra
	// $match stage used to shard the load between several instances.
	DivideProcessingExpression bson.D `json:"-"`

	ReadFromSecondary bool `json:"-"`

	// Latency tuning. The Node.js version drains its queues on fixed
	// setTimeout cycles (150 ms for realtime data, 333 ms for historical);
	// here the writers flush as soon as a batch fills or the linger time
	// expires, whichever comes first.
	RtWriteLinger     time.Duration `json:"-"`
	RtWriteMaxBatch   int           `json:"-"`
	HistWriteLinger   time.Duration `json:"-"`
	HistWriteMaxBatch int           `json:"-"`
	SoeWriteLinger    time.Duration `json:"-"`
	SoeWriteMaxBatch  int           `json:"-"`
	Workers           int           `json:"-"`
	ChangeQueueSize   int           `json:"-"`
	WriteQueueSize    int           `json:"-"`
	CSBatchSize       int32         `json:"-"`
	CSMaxAwaitTime    time.Duration `json:"-"`

	// Instrumentation
	MetricsPort        int           `json:"-"`
	MetricsLogInterval time.Duration `json:"-"`
	MetricsFile        string        `json:"-"`
}

func envStr(name string) string { return strings.TrimSpace(os.Getenv(EnvPrefix + name)) }

func envInt(name string, def int) int {
	s := envStr(name)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		Log(LogLevelMin, "Config - %s%s should be a number, using default %d", EnvPrefix, name, def)
		return def
	}
	return v
}

func envDurationMs(name string, def time.Duration) time.Duration {
	s := envStr(name)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		Log(LogLevelMin, "Config - %s%s should be a number of milliseconds, using default %v", EnvPrefix, name, def)
		return def
	}
	return time.Duration(v) * time.Millisecond
}

func envBool(name string) bool {
	return strings.EqualFold(envStr(name), "true")
}

// LoadConfig mirrors load-config.js: command line arguments win over
// environment variables, which win over the defaults.
func LoadConfig() Config {
	cfg := Config{
		Instance: 1,
		LogLevel: LogLevelNormal,
	}

	// instance number
	cfg.Instance = envInt("INSTANCE", 1)
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			cfg.Instance = v
		} else {
			fmt.Println("Instance parameter should be a number!")
			os.Exit(2)
		}
	}

	// log level
	cfg.LogLevel = envInt("LOGLEVEL", LogLevelNormal)
	if len(os.Args) > 2 {
		if v, err := strconv.Atoi(os.Args[2]); err == nil {
			cfg.LogLevel = v
		} else {
			fmt.Println("Log Level parameter should be a number!")
			os.Exit(2)
		}
	}
	setLogLevel(cfg.LogLevel)

	// config file
	cfgFile := findConfigFile()
	if len(os.Args) > 3 && strings.TrimSpace(os.Args[3]) != "" {
		cfgFile = os.Args[3]
	}
	Log(LogLevelMin, "Config - Config File: %s", cfgFile)
	raw, err := os.ReadFile(cfgFile)
	if err != nil {
		Log(LogLevelMin, "Config - Error: config file not found!")
		time.Sleep(300 * time.Millisecond)
		os.Exit(1)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		Log(LogLevelMin, "Config - Error parsing config file: %v", err)
		time.Sleep(300 * time.Millisecond)
		os.Exit(1)
	}
	cfg.MongoConnectionString = strings.TrimSpace(cfg.MongoConnectionString)
	cfg.MongoDatabaseName = strings.TrimSpace(cfg.MongoDatabaseName)
	cfg.NodeName = strings.TrimSpace(cfg.NodeName)
	if cfg.MongoConnectionString == "" {
		Log(LogLevelMin, "Error reading config file.")
		time.Sleep(300 * time.Millisecond)
		os.Exit(1)
	}
	if cfg.MongoDatabaseName == "" {
		cfg.MongoDatabaseName = "json_scada"
	}

	// optional $match stage appended to the change stream pipeline
	if exp := envStr("DIVIDE_EXP"); exp != "" {
		var d bson.D
		if err := bson.UnmarshalExtJSON([]byte(exp), false, &d); err != nil {
			Log(LogLevelMin, "Divide Processing Expression: ERROR! %v", err)
			time.Sleep(300 * time.Millisecond)
			os.Exit(1)
		}
		cfg.DivideProcessingExpression = d
		Log(LogLevelMin, "Divide Processing Expression: %s", exp)
	}

	cfg.ReadFromSecondary = envBool("READ_FROM_SECONDARY")
	if cfg.ReadFromSecondary {
		Log(LogLevelMin, "Read From Secondary (Preferred): TRUE")
	}

	cfg.SQLFilesDir = envStr("SQL_FILES_PATH")
	if cfg.SQLFilesDir == "" {
		cfg.SQLFilesDir = findSQLDir()
	}

	// Latency tuning knobs
	cfg.RtWriteLinger = envDurationMs("RT_WRITE_LINGER_MS", 20*time.Millisecond)
	cfg.RtWriteMaxBatch = envInt("RT_WRITE_MAX_BATCH", 2000)
	cfg.HistWriteLinger = envDurationMs("HIST_WRITE_LINGER_MS", 250*time.Millisecond)
	cfg.HistWriteMaxBatch = envInt("HIST_WRITE_MAX_BATCH", 5000)
	cfg.SoeWriteLinger = envDurationMs("SOE_WRITE_LINGER_MS", 20*time.Millisecond)
	cfg.SoeWriteMaxBatch = envInt("SOE_WRITE_MAX_BATCH", 500)
	cfg.Workers = envInt("WORKERS", 4)
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	cfg.ChangeQueueSize = envInt("CHANGE_QUEUE_SIZE", 65536)
	cfg.WriteQueueSize = envInt("WRITE_QUEUE_SIZE", 65536)
	cfg.CSBatchSize = int32(envInt("CS_BATCH_SIZE", 1000))
	cfg.CSMaxAwaitTime = envDurationMs("CS_MAX_AWAIT_MS", 200*time.Millisecond)

	// Instrumentation
	cfg.MetricsPort = envInt("METRICS_PORT", 0)
	cfg.MetricsLogInterval = time.Duration(envInt("METRICS_LOG_INTERVAL", 60)) * time.Second
	cfg.MetricsFile = envStr("METRICS_FILE")

	Log(LogLevelMin, "Config - %s Version %s", AppMsg, AppVersion)
	Log(LogLevelMin, "Config - Instance: %d", cfg.Instance)
	Log(LogLevelMin, "Config - Log level: %d", cfg.LogLevel)
	Log(LogLevelMin, "Config - SQL files dir: %s", cfg.SQLFilesDir)
	Log(LogLevelMin, "Config - Workers: %d, RT linger: %v, Hist linger: %v",
		cfg.Workers, cfg.RtWriteLinger, cfg.HistWriteLinger)

	return cfg
}

// findConfigFile probes the usual locations, tolerating both a run from the
// source folder (../../conf) and from an installed bin folder (../conf).
func findConfigFile() string {
	if v := strings.TrimSpace(os.Getenv("JS_CONFIG_FILE")); v != "" {
		return v
	}
	candidates := []string{
		filepath.Join("..", "..", "conf", "json-scada.json"),
		filepath.Join("..", "conf", "json-scada.json"),
		filepath.Join("conf", "json-scada.json"),
		filepath.Join("c:\\", "json-scada", "conf", "json-scada.json"),
		filepath.Join("/", "json-scada", "conf", "json-scada.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// findSQLDir locates the folder watched by src/process_pg_sql.
func findSQLDir() string {
	candidates := []string{
		filepath.Join("..", "..", "sql"),
		filepath.Join("..", "sql"),
		"sql",
		filepath.Join("c:\\", "json-scada", "sql"),
		filepath.Join("/", "sql"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return candidates[0]
}

// mongoClientOptions builds the driver options from the config, matching the
// TLS handling of the other {json:scada} drivers.
func (cfg Config) mongoClientOptions() *options.ClientOptions {
	connStr := cfg.MongoConnectionString
	add := func(k, v string) {
		sep := "&"
		if !strings.Contains(connStr, "?") {
			sep = "?"
		}
		connStr += sep + k + "=" + url.QueryEscape(v)
	}
	if cfg.TLSCaPemFile != "" || cfg.TLSClientPemFile != "" {
		add("tls", "true")
	}
	if cfg.TLSCaPemFile != "" {
		add("tlsCAFile", cfg.TLSCaPemFile)
	}
	if cfg.TLSClientPemFile != "" {
		add("tlsCertificateKeyFile", cfg.TLSClientPemFile)
	}
	if cfg.TLSClientKeyPassword != "" {
		add("tlsCertificateKeyFilePassword", cfg.TLSClientKeyPassword)
	}
	if cfg.TLSInsecure || cfg.TLSAllowChainErrors {
		add("tlsInsecure", "true")
	}
	if cfg.TLSAllowInvalidHostnames {
		add("tlsAllowInvalidHostnames", "true")
	}

	opts := options.Client().ApplyURI(connStr).
		SetAppName(AppName + " Version:" + AppVersion + " Instance:" + strconv.Itoa(cfg.Instance)).
		SetMaxPoolSize(20)
	if cfg.ReadFromSecondary {
		opts = opts.SetReadPreference(readpref.SecondaryPreferred())
	} else {
		opts = opts.SetReadPreference(readpref.Primary())
	}
	return opts
}
