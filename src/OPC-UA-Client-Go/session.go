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

// Endpoint discovery and session establishment.
// Port of OPCUAClient.ConsoleClient() in AsduReceiveHandler.cs.

package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

const (
	// retryPeriod is the pause between session creation attempts, as in
	// the C# driver's retry loop.
	retryPeriod = 5 * time.Second

	// reconnectPeriod is how often gopcua retries a lost connection; the
	// C# driver passes the same value to SessionReconnectHandler.
	reconnectPeriod = 10 * time.Second

	// reconnectGiveUpPeriod is how long the client may stay out of the
	// Connected state before it is torn down and rebuilt. gopcua's
	// auto-reconnect retries one endpoint URL forever, so without this the
	// driver could never fail over to the next configured endpoint. Part of
	// deviation D11.
	reconnectGiveUpPeriod = 60 * time.Second

	// maxDiscoveryBackoff caps the wait between repeated failed attempts to
	// discover a namespace.
	maxDiscoveryBackoff = 5 * time.Minute

	// sessionTimeout matches the value the C# driver passes to
	// Session.Create.
	sessionTimeout = 60 * time.Second
)

// discoveryBackoff spaces out repeated discovery failures, from retryPeriod
// up to maxDiscoveryBackoff.
func discoveryBackoff(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	wait := retryPeriod
	for i := 1; i < failures && wait < maxDiscoveryBackoff; i++ {
		wait *= 2
	}
	return min(wait, maxDiscoveryBackoff)
}

// certDir is where generated certificates and the server trust list live.
var certDir = filepath.Join("..", "conf", "opcua")

// connectionLoop owns one OPC UA connection for the lifetime of the
// process: it builds a client, connects, sets up acquisition, and rebuilds
// everything when the session is lost.
//
// This replaces both the C# retry loop inside ConsoleClient() and the
// watchdog in Program.cs that restarted a failed connection thread. Doing
// it in one goroutine makes it impossible to run two clients for the same
// connection, which is the race the C# watchdog comments warn about.
func connectionLoop(ctx context.Context, cfg jsconfig.Config, conn *OPCUAConnection) {
	appCfg := readUAConfigXML(conn.ConfigFileName)
	attempt := 0
	discoveryFailures := 0

	for ctx.Err() == nil {
		cli, err := connectOnce(ctx, conn, appCfg, attempt)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "%s - FATAL: error creating session! %v", conn.Name, err)
			attempt++ // advance to the next configured endpoint URL
			if !sleepCtx(ctx, retryPeriod) {
				return
			}
			continue
		}

		jslog.Log(jslog.LevelNoLog, "%s - Session created successfully.", conn.Name)
		conn.setClient(cli)

		// Browsing must precede the subscriptions: it appends the
		// discovered items to conn.ListMon, which is what they subscribe.
		//
		// deviation D18: discovery that does not complete must not be
		// published:
		// subscribing a partial namespace would leave most points without a
		// tag until the driver is restarted. Rebuild the session instead.
		discovered := true
		if conn.AutoCreateTags {
			browsed, err := browseFullAddressSpace(ctx, cli, conn,
				ua.NewNumericNodeID(0, id.ObjectsFolder))
			if err != nil {
				jslog.Log(jslog.LevelNoLog, "%s - Error browsing the namespace: %v", conn.Name, err)
				discovered = false
			} else if err := autotagPass(ctx, cli, conn, browsed); err != nil {
				jslog.Log(jslog.LevelNoLog, "%s - Tag discovery incomplete: %v", conn.Name, err)
				discovered = false

				// A server that drops the connection rather than answering
				// an oversized value read would fail the same way forever.
				// Ask for fewer values at a time on the next attempt.
				if errors.Is(err, errValueReadTooLarge) {
					if n, shrank := conn.ShrinkValueReadChunk(); shrank {
						jslog.Log(jslog.LevelNoLog,
							"%s - Reading fewer values at a time from now on: %d", conn.Name, n)
					} else {
						jslog.Log(jslog.LevelNoLog,
							"%s - Already reading only %d values at a time and the server still "+
								"drops the connection; the namespace cannot be discovered.", conn.Name, n)
					}
				}
			}
		}

		if !discovered {
			discoveryFailures++
			conn.setClient(nil)
			closeCtx, cancelDisc := context.WithTimeout(context.Background(), 5*time.Second)
			_ = cli.Close(closeCtx)
			cancelDisc()
			if ctx.Err() != nil {
				return
			}
			// Read the tags back, including any the partial pass created,
			// so the next attempt starts from the same state a fresh
			// process would.
			if err := reloadConnectionTags(ctx, cfg, conn); err != nil {
				jslog.Log(jslog.LevelNoLog, "%s - Error reloading tags before retry: %v", conn.Name, err)
			}
			// Back off: a discovery that keeps failing must not hammer the
			// server. Each failed attempt leaves a session behind when the
			// connection died before it could be closed, and some servers
			// then refuse new ones entirely.
			wait := discoveryBackoff(discoveryFailures)
			jslog.Log(jslog.LevelNoLog,
				"%s - Restarting the connection to discover the namespace again in %v...", conn.Name, wait)
			attempt++
			if !sleepCtx(ctx, wait) {
				return
			}
			continue
		}

		discoveryFailures = 0
		setupSubscriptions(ctx, cli, conn)

		jslog.Log(jslog.LevelNoLog, "%s - Running...", conn.Name)

		reason := watchClient(ctx, conn, cli)
		conn.setClient(nil)
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = cli.Close(closeCtx)
		cancel()

		if ctx.Err() != nil {
			return
		}
		jslog.Log(jslog.LevelNoLog, "%s - Connection lost (%s), reconnecting...", conn.Name, reason)
		attempt++
		if !sleepCtx(ctx, retryPeriod) {
			return
		}
	}
}

// connectOnce builds and connects a client against one of the configured
// endpoint URLs.
func connectOnce(ctx context.Context, conn *OPCUAConnection, appCfg uaAppConfig, attempt int) (*opcua.Client, error) {
	// parity: the C# driver rotates through the endpoint URLs so failover
	// to a redundant server happens on retry.
	endpointURL := conn.EndpointURLs[attempt%len(conn.EndpointURLs)]

	ep, err := selectEndpoint(ctx, conn, endpointURL)
	if err != nil {
		return nil, err
	}

	opts, err := clientOptions(conn, appCfg, ep)
	if err != nil {
		return nil, err
	}

	jslog.Log(jslog.LevelNoLog, "%s - Create a session with OPC UA server.", conn.Name)
	cli, err := opcua.NewClient(ep.EndpointURL, opts...)
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(conn.TimeoutMs)*time.Millisecond)
	defer cancel()
	if err := cli.Connect(dialCtx); err != nil {
		closeCtx, ccancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = cli.Close(closeCtx)
		ccancel()
		return nil, err
	}
	return cli, nil
}

// selectEndpoint discovers the server's endpoints and picks one with the
// C# driver's preference order:
//
//  1. exact security policy AND security mode
//  2. first with a matching security mode
//  3. the first endpoint offered
//
// Discovery failure falls back to an endpoint assembled from the connection
// document. gopcua's own opcua.SelectEndpoint is deliberately not used: it
// orders by SecurityLevel and would pick a different endpoint than the C#
// driver on the same server (deviation D5).
func selectEndpoint(ctx context.Context, conn *OPCUAConnection, endpointURL string) (*ua.EndpointDescription, error) {
	secMode := conn.SecurityMode
	secPolicy := conn.SecurityPolicy

	jslog.Log(jslog.LevelNoLog, "%s - Discovering endpoints from server...", conn.Name)
	if !conn.UseSecurity {
		jslog.Log(jslog.LevelNoLog, "%s - Warning: Security is disabled, will attempt to use unsecure endpoint.", conn.Name)
		secMode = "None"
		secPolicy = "None"
	}

	wantPolicy := ua.FormatSecurityPolicyURI(secPolicy)
	wantMode := ua.MessageSecurityModeFromString(secMode)

	var selected *ua.EndpointDescription

	discCtx, cancel := context.WithTimeout(ctx, time.Duration(conn.TimeoutMs)*time.Millisecond)
	eps, err := opcua.GetEndpoints(discCtx, endpointURL)
	cancel()
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "%s - Warning: Could not discover endpoints: %v", conn.Name, err)
	} else if len(eps) > 0 {
		jslog.Log(jslog.LevelNoLog, "%s - Found %d endpoints from server.", conn.Name, len(eps))

		for _, ep := range eps {
			if ep.SecurityPolicyURI == wantPolicy && ep.SecurityMode == wantMode {
				selected = ep
				jslog.Log(jslog.LevelNoLog, "%s - Selected discovered endpoint matching security policy: %s and mode: %v",
					conn.Name, policyShortName(ep.SecurityPolicyURI), ep.SecurityMode)
				break
			}
		}
		if selected == nil {
			for _, ep := range eps {
				if ep.SecurityMode == wantMode {
					selected = ep
					jslog.Log(jslog.LevelNoLog, "%s - Selected discovered endpoint with matching security mode: %v",
						conn.Name, ep.SecurityMode)
					break
				}
			}
		}
		if selected == nil {
			selected = eps[0]
			jslog.Log(jslog.LevelNoLog, "%s - Using first discovered endpoint: %s | Mode: %v",
				conn.Name, policyShortName(selected.SecurityPolicyURI), selected.SecurityMode)
		}
	}

	if selected == nil {
		jslog.Log(jslog.LevelNoLog, "%s - Assembling endpoint directly from configuration.", conn.Name)
		selected = &ua.EndpointDescription{
			EndpointURL:       endpointURL,
			SecurityMode:      wantMode,
			SecurityPolicyURI: wantPolicy,
			UserIdentityTokens: []*ua.UserTokenPolicy{
				{TokenType: ua.UserTokenTypeAnonymous, PolicyID: "Anonymous"},
				{TokenType: ua.UserTokenTypeUserName, PolicyID: "UserName"},
				{TokenType: ua.UserTokenTypeCertificate, PolicyID: "Certificate"},
			},
		}
		jslog.Log(jslog.LevelNoLog, "%s - Assembled endpoint: %s | SecurityMode: %v | SecurityPolicy: %s",
			conn.Name, selected.EndpointURL, selected.SecurityMode, policyShortName(selected.SecurityPolicyURI))
	}

	// A discovered endpoint may advertise a host name this driver cannot
	// resolve. The configured URL is the one the operator can reach, so
	// keep it and take only the security settings from discovery.
	if selected.EndpointURL != endpointURL {
		jslog.Log(jslog.LevelDetailed, "%s - Server advertises %s; connecting to the configured %s instead.",
			conn.Name, selected.EndpointURL, endpointURL)
		selected.EndpointURL = endpointURL
	}

	jslog.Log(jslog.LevelNoLog, "%s - Selected endpoint uses: %s", conn.Name, policyShortName(selected.SecurityPolicyURI))
	if len(selected.UserIdentityTokens) > 0 {
		jslog.Log(jslog.LevelNoLog, "%s - Endpoint supports the following authentication methods:", conn.Name)
		for _, t := range selected.UserIdentityTokens {
			jslog.Log(jslog.LevelNoLog, "%s -   - %v (PolicyId: %s)", conn.Name, t.TokenType, t.PolicyID)
		}
	} else {
		jslog.Log(jslog.LevelNoLog, "%s - WARNING: Endpoint has no UserIdentityTokens defined!", conn.Name)
	}

	if !conn.AutoAcceptUntrustedCertificates && selected.SecurityMode != ua.MessageSecurityModeNone {
		if err := validateServerCert(selected.ServerCertificate, filepath.Join(certDir, "trusted"), conn.Name); err != nil {
			return nil, err
		}
	}

	return selected, nil
}

// clientOptions assembles the gopcua options from the connection document.
//
// Option order matters: the Auth* options create the user identity token
// and SecurityFromEndpoint fills in its PolicyID from the endpoint, so the
// Auth* options must come first.
func clientOptions(conn *OPCUAConnection, appCfg uaAppConfig, ep *ua.EndpointDescription) ([]opcua.Option, error) {
	appURI := appCfg.ApplicationURI

	// The application instance certificate, needed for any secure mode.
	var appCert *keyPair
	secure := conn.UseSecurity && ep.SecurityMode != ua.MessageSecurityModeNone
	if secure {
		var err error
		if conn.LocalCertFilePath != "" {
			appCert, err = loadKeyPair(conn.LocalCertFilePath, conn.Passphrase)
			if err != nil {
				// parity: the C# driver treats a bad local certificate file
				// as fatal.
				jslog.Fatal("%s - FATAL: error in local certificate file! %v", conn.Name, err)
			}
			jslog.Log(jslog.LevelBasic, "%s - Using application certificate %s", conn.Name, conn.LocalCertFilePath)
		} else {
			appCert, err = ensureClientCert(certDir, appURI, appCfg.ApplicationName)
			if err != nil {
				return nil, err
			}
		}
		if appCert.AppURI != "" {
			// The session's ApplicationUri must match the certificate's
			// URI SAN or the server answers BadCertificateUriInvalid.
			appURI = appCert.AppURI
		}
	}

	opts := []opcua.Option{
		opcua.ApplicationName(appCfg.ApplicationName),
		opcua.ApplicationURI(appURI),
		opcua.ProductURI(appCfg.ProductURI),
		opcua.SessionName(appCfg.ApplicationName + " " + conn.Name),
		opcua.SessionTimeout(sessionTimeout),
		opcua.RequestTimeout(time.Duration(conn.TimeoutMs) * time.Millisecond),
		opcua.AutoReconnect(true),
		opcua.ReconnectInterval(reconnectPeriod),
		// deviation D17: transport limits are deliberately left at
		// gopcua's defaults,
		// which advertise "no client limit" in the UACP handshake and let
		// the server's own limits apply.
		//
		// Do not copy the C# TransportQuotas numbers here. Setting
		// MaxMessageSize to the .NET default of 4 MB advertises a hard cap
		// to the server, and a server whose response exceeds it answers
		// BadTCPMessageTooLarge and drops the connection instead of
		// truncating — which is what a real server (Sterfive's public demo)
		// does when reading the values of a few hundred nodes with large
		// arrays. The .NET stack applies its quota differently, so the
		// number is not portable.
		opcua.StateChangedFunc(func(s opcua.ConnState) {
			// Called synchronously by the client: log only, never block.
			jslog.Log(jslog.LevelDetailed, "%s - connection state: %v", conn.Name, s)
		}),
	}

	if secure {
		opts = append(opts,
			opcua.Certificate(appCert.CertDER),
			opcua.PrivateKey(appCert.Key),
		)
	}

	// User identity. These must precede SecurityFromEndpoint.
	tokenType := ua.UserTokenTypeAnonymous
	switch {
	case conn.Username != "":
		jslog.Log(jslog.LevelNoLog, "%s - Using username/password authentication for user: %s", conn.Name, conn.Username)
		tokenType = ua.UserTokenTypeUserName
		opts = append(opts, opcua.AuthUsername(conn.Username, conn.Password))

	case conn.PfxFilePath != "":
		jslog.Log(jslog.LevelNoLog, "%s - Using certificate authentication from: %s", conn.Name, conn.PfxFilePath)
		userCert, err := loadKeyPair(conn.PfxFilePath, conn.Passphrase)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "%s - ERROR: Failed to load certificate: %v", conn.Name, err)
			return nil, err
		}
		tokenType = ua.UserTokenTypeCertificate
		opts = append(opts,
			opcua.AuthCertificate(userCert.CertDER),
			opcua.AuthPrivateKey(userCert.Key),
		)

	default:
		jslog.Log(jslog.LevelNoLog, "%s - Using anonymous authentication.", conn.Name)
		opts = append(opts, opcua.AuthAnonymous())
	}

	// Takes the policy, mode, server certificate and the matching token
	// PolicyID from the endpoint.
	opts = append(opts, opcua.SecurityFromEndpoint(ep, tokenType))

	return opts, nil
}

// watchClient blocks while the session is usable and returns why it is not.
//
// The state is polled rather than read from opcua.StateChangedCh, because
// the client sends to that channel synchronously: a slow or unread reader
// would stall the client itself.
func watchClient(ctx context.Context, conn *OPCUAConnection, cli *opcua.Client) string {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastConnected := time.Now()
	var lastWaitLog time.Time
	for {
		select {
		case <-ctx.Done():
			return "shutting down"
		case <-ticker.C:
		}

		switch cli.State() {
		case opcua.Connected:
			lastConnected = time.Now()

		case opcua.Closed:
			// gopcua gave up by itself, e.g. the server refused the
			// connection outright.
			return "session closed"

		default:
			// Disconnected or Reconnecting: gopcua is retrying this one
			// endpoint URL. Give it a while, then rebuild so the next
			// configured endpoint gets a turn.
			if time.Since(lastConnected) > reconnectGiveUpPeriod {
				return "reconnect timed out"
			}
			// Throttled: a whole give-up window at one line per second
			// would bury everything else in the log.
			if time.Since(lastWaitLog) >= 10*time.Second {
				jslog.Log(jslog.LevelDetailed, "%s - waiting for reconnection, %.0fs since last connected...",
					conn.Name, time.Since(lastConnected).Seconds())
				lastWaitLog = time.Now()
			}
		}
	}
}

// policyShortName renders a security policy URI the way the C# logs do,
// keeping only the fragment after '#'.
func policyShortName(uri string) string {
	if i := strings.LastIndex(uri, "#"); i >= 0 {
		return uri[i+1:]
	}
	return uri
}

// sleepCtx waits for d, returning false when the context is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
