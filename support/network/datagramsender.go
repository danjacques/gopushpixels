// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package network

import (
	"io"
	"net"
)

// DatagramSender exposes an interface which sends individual datagrams.
type DatagramSender interface {
	io.Closer
	SendDatagram(b []byte) error

	// MaxDatagramSize returns the maximum allowed packet size.
	//
	// This value is advisory; the DatagramSender is not repsonsible for enforcing
	// this size.
	MaxDatagramSize() int
}

// UDPDatagramSender returns a DatagramSender that sends through conn.
//
// UDPDatagramSender takes ownership of conn, and will close it when Close is
// called.
func UDPDatagramSender(conn *net.UDPConn) DatagramSender {
	return &udpDatagramSender{conn}
}

type udpDatagramSender struct {
	// conn is the underlying UDP connectiopn.
	conn *net.UDPConn
}

// SendDatagram implements DatagramSender.
func (uds *udpDatagramSender) SendDatagram(b []byte) error {
	_, _, err := uds.conn.WriteMsgUDP(b, nil, nil)
	return err
}

func (uds *udpDatagramSender) MaxDatagramSize() int { return MaxUDPSize }
func (uds *udpDatagramSender) Close() error         { return uds.conn.Close() }

// ResilientDatagramSender is a DatagramSender that automatically reconnects
// on failure.
type ResilientDatagramSender struct {
	// Factory genrates and connects a new DatagramSender. On success, the
	// ResilientDatagramSender will take ownership of the result.
	Factory func() (DatagramSender, error)

	// base is the currently-connected DatagramSender, or nil if none is currently
	// connected.
	base DatagramSender
}

var _ DatagramSender = (*ResilientDatagramSender)(nil)

// MaxDatagramSize implements DatagramSender.
func (rds *ResilientDatagramSender) MaxDatagramSize() int { return rds.base.MaxDatagramSize() }

// Connect causes rds to try and open a new connection.
//
// If Connect fails, and rds already has an open connection, the open connection
// will be left in-tact. If Connect succeeds, the previous connection will be
// closed.
func (rds *ResilientDatagramSender) Connect() error {
	base, err := rds.Factory()
	if err != nil {
		return err
	}

	// Replace the current one, if applicable.
	if rds.base != nil {
		_ = rds.Close()
	}
	rds.base = base
	return nil
}

// Close closes the current connection, if one is open.
//
// If no connection is open, Close will do nothing.
func (rds *ResilientDatagramSender) Close() error {
	if rds.base == nil {
		return nil
	}

	err := rds.base.Close()
	rds.base = nil
	return err
}

// SendDatagram calls the corresponding call on rds's underlying connection.
//
// If rds is not currently connected, rds will attempt to reconnect.
func (rds *ResilientDatagramSender) SendDatagram(b []byte) error {
	if err := rds.ensureConnected(); err != nil {
		return err
	}

	if err := rds.base.SendDatagram(b); err != nil {
		_ = rds.Close()
		return err
	}
	return nil
}

func (rds *ResilientDatagramSender) ensureConnected() error {
	// IF we aren't currently connected, try and connect.
	if rds.base != nil {
		return nil
	}
	return rds.Connect()
}
