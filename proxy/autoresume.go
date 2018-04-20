// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package proxy

import (
	"context"
	"time"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/logging"
)

// AutoResumeListener automatically resumes a paused playback stream if:
// (a) the Proxy has received at least one packet, and
// (b) the Proxy has not received packets after a specified Delay.
//
// This can be used to automatically re-enable playback after pausing the stream
// for demonstration purposes.
type AutoResumeListener struct {
	// ProxyManager is the proxy manager.
	ProxyManager *Manager
	// OnDelay is a callback function that will trigger after a specified delay.
	OnDelay func(context.Context)
	// Delay is the amount of tiem to wait after the last packet was received.
	Delay time.Duration
	// Logger is the logger instance to use. If nil, no logging will be done.
	Logger logging.L

	cancelFunc context.CancelFunc

	signalC   chan struct{}
	finishedC chan struct{}

	// logger is resolved from Logger.
	logger logging.L
}

// Start starts the AutoResumeListener. It will run until its Context is
// cancelled.
func (l *AutoResumeListener) Start(c context.Context) {
	c, cancelFunc := context.WithCancel(c)

	l.cancelFunc = cancelFunc
	l.signalC = make(chan struct{}, 1)
	l.finishedC = make(chan struct{})
	l.logger = logging.Must(l.Logger)

	go func() {
		defer close(l.finishedC)
		l.handleAutoResume(c)
	}()
}

// Stop stops the auto resume listener, unregistering it.
func (l *AutoResumeListener) Stop() {
	l.cancelFunc()
	<-l.finishedC
}

// ReceivePacket implements Listener.
func (l *AutoResumeListener) ReceivePacket(d device.D, pkt *protocol.Packet, forwarded bool) {
	// Notify our listener that we've received a packet.
	l.signalC <- struct{}{}
}

func (l *AutoResumeListener) handleAutoResume(c context.Context) {
	var timer *time.Timer
	defer func() {
		l.ProxyManager.RemoveListener(l)
		if timer != nil {
			timer.Stop()
		}
	}()

	// Register as a listener with our Proxy.
	l.ProxyManager.AddListener(l)

	for {
		var timerC <-chan time.Time
		if timer != nil {
			timerC = timer.C
		}

		// Wait until the next event.
		select {
		case <-c.Done():
			return

		case <-l.signalC:
			// Signal that we've received another packet; start/reset our timer.
			if timer == nil {
				l.logger.Infof("Auto resume received first packet, starting countdown (%s)...", l.Delay)
				timer = time.NewTimer(l.Delay)
			} else {
				// Stop the previous timer and reset.
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(l.Delay)
			}

		case <-timerC:
			// Notify that we're done.
			l.logger.Info("Auto resume hit delay (%s); triggering.", l.Delay)
			go l.OnDelay(c)
			return
		}
	}
}
