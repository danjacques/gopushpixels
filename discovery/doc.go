// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

// Package discovery implements UDP multicast discovery functionality.
//
// Most users will want to use a Registry, which keeps track of observed remote
// devices and instantiates Remote instances for each such device.
//
// Transmitter and Listener are low-level discovery primitives that can
// broadcast discovery packets and receive discovery packets respectively.
package discovery
