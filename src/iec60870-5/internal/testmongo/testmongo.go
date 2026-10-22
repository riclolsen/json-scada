/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - test MongoDB
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Test support only: provides a throwaway MongoDB for tests that need one.
 * The server given by JS_TEST_MONGO_URI is used when set; otherwise the
 * bundled mongod of platform-windows/platform-linux (or JS_TEST_MONGOD) is
 * started on a temporary dbpath and a free port. Tests skip when neither is
 * available.
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

package testmongo

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// mongodPath returns the bundled mongod binary, or "" when not present.
func mongodPath() string {
	if p := os.Getenv("JS_TEST_MONGOD"); p != "" {
		return p
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// internal/testmongo -> repository root
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	cands := []string{
		filepath.Join(root, "platform-windows", "mongodb-runtime", "bin", "mongod.exe"),
		filepath.Join(root, "platform-linux", "mongodb-runtime", "bin", "mongod"),
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %s", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// Start returns the URI of a reachable MongoDB and a connected client,
// starting the bundled mongod when JS_TEST_MONGO_URI is not set. Skips the
// test when no server can be had. Everything is torn down on test cleanup.
func Start(t *testing.T) (string, *mongo.Client) {
	t.Helper()
	uri := os.Getenv("JS_TEST_MONGO_URI")
	if uri == "" {
		bin := mongodPath()
		if bin == "" {
			t.Skip("no mongod available (set JS_TEST_MONGOD or JS_TEST_MONGO_URI)")
		}
		port := freePort(t)
		cmd := exec.Command(bin, "--dbpath", t.TempDir(), "--port", strconv.Itoa(port),
			"--bind_ip", "127.0.0.1")
		if err := cmd.Start(); err != nil {
			t.Skipf("cannot start mongod: %s", err)
		}
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		})
		uri = "mongodb://127.0.0.1:" + strconv.Itoa(port)
	}

	var lastErr error
	deadline := time.Now().Add(60 * time.Second)
	for {
		c, err := mongo.Connect(options.Client().ApplyURI(uri).
			SetServerSelectionTimeout(2 * time.Second))
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = c.Ping(ctx, nil)
			cancel()
			if err == nil {
				t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
				return uri, c
			}
			_ = c.Disconnect(context.Background())
		}
		lastErr = err
		if time.Now().After(deadline) {
			t.Skipf("MongoDB not reachable at %s: %v", uri, lastErr)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// DB returns a freshly dropped database on the test MongoDB.
func DB(t *testing.T, name string) (string, *mongo.Database) {
	t.Helper()
	uri, client := Start(t)
	db := client.Database(name)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = db.Drop(ctx)
	return uri, db
}
