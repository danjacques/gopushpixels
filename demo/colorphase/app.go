// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

// Package colorphase defines the logic for the "colorphase" demo app.
//
// This app listens for any PixelPusher devices and broadcasts a color cycling
// animation to each, fading in and out of intensity of various colors.
//
// This demonstrates how to listen for connections, register them with a
// discovery Registry, generate pixel state mutations, and send those mutations
// to the target device.
package colorphase

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/discovery"
	"github.com/danjacques/gopushpixels/pixel"
)

var (
	fps = flag.Int("fps", 30, "FPS that colors will be cycled.")
)

// Main is the main entry point.
func Main() {
	flag.Parse()

	// Calculate sleep interval from FPS.
	sleepInterval := time.Second / (time.Second * time.Duration(*fps))

	// Start discovery.
	var l discovery.Listener
	conn, err := discovery.DefaultListenerConn().ListenMulticastUDP4()
	if err != nil {
		log.Fatalf("Couldn't listen for discovery packets: %s", err)
	}
	defer conn.Close()
	if err := l.Start(conn); err != nil {
		log.Fatalf("Couldn't start discovery listener: %s", err)
	}

	var reg discovery.Registry
	err = discovery.ListenAndRegister(context.Background(), &l, &reg, func(d device.D) error {
		go cycleColors(d, sleepInterval)
		return nil
	})
	if err != nil {
		log.Fatalf("Error registering devices: %s", err)
	}
}

func cycleColors(d device.D, sleepInterval time.Duration) {
	dh := d.DiscoveryHeaders()
	log.Printf("Cycling on device %s: %s", d.ID(), dh)

	sender, err := d.Sender()
	if err != nil {
		log.Printf("Could't create sender for device %q: %s", d.ID(), err)
		return
	}
	defer sender.Close()

	// Create a mutable state for the device.
	var m device.Mutable
	m.Initialize(dh)

	var c cycler
	for {
		// Cycle colors.
		//
		// First, shift all pixels towards the end.
		next := c.Next()
		for s := 0; s < m.NumStrips(); s++ {
			for i := m.PixelsPerStrip() - 1; i > 0; i-- {
				m.SetPixel(s, i, m.GetPixel(s, i-1))
			}
			m.SetPixel(s, 0, next)
		}

		// Send any update packets.
		if pkt := m.SyncPacket(); pkt != nil {
			if err := sender.SendPacket(pkt); err != nil {
				log.Printf("Couldn't send packet to device %q: %s", d.ID(), err)
				return
			}
		}

		time.Sleep(sleepInterval)
	}
}

// 101 110 011 100 010 001
const cyclerMask = uint(0x2E711)

type cycler struct {
	v    int
	mask uint
}

func (c *cycler) Next() (p pixel.P) {
	if c.mask == 0 {
		c.mask = cyclerMask
	}

	// Select our intensity. >0xFF fades downwards towards 0.
	v := c.v
	if v > 0xFF {
		v = 0xFF - v
	}

	// Set masked colors.
	if c.mask&0x01 != 0 {
		p.Red = byte(v)
	}
	if c.mask&0x02 != 0 {
		p.Green = byte(v)
	}
	if c.mask&0x04 != 0 {
		p.Blue = byte(v)
	}

	// Cycle.
	c.v++
	if c.v > 0x1FF {
		c.v = 0
		c.mask >>= 3
	}
	return
}
