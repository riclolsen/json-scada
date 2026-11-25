/*
 * DNP3 Client and Server Protocol drivers for {json:scada}, in Go.
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

// The peer of a session on a bus.
//
// A session describes its peer from the connection its channel produced, and
// a multidrop station's connection is a view of the bus line with no address
// of its own. Without this every security event of a session, and the MAC
// lookup and the peer history behind them, would have no peer.

package dnp3util

import (
	"context"
	"io"

	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/secevent"
)

type peerChannel struct {
	channel.Channel
	line *Counters
}

// withLinePeer makes the connections of a bus station describe the bus line's
// peer.
func withLinePeer(ch channel.Channel, line *Counters) channel.Channel {
	return &peerChannel{Channel: ch, line: line}
}

func (c *peerChannel) Connect(ctx context.Context) (io.ReadWriteCloser, error) {
	conn, err := c.Channel.Connect(ctx)
	if err != nil {
		return nil, err
	}
	p, _ := c.line.LinePeer()
	return &peerConn{ReadWriteCloser: conn, peer: p}, nil
}

type peerConn struct {
	io.ReadWriteCloser
	peer secevent.PeerInfo
}

// PeerInfo implements secevent.PeerInfoer.
func (c *peerConn) PeerInfo() secevent.PeerInfo { return c.peer }
