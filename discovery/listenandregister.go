// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package discovery

import (
	"context"

	"github.com/danjacques/gopushpixels/device"
)

// ListenAndRegister is a convenience function to listen for device discovery
// packets on l and register all of these devices with reg.
//
// ListenAndRegister will run until c is cancelled, or the fn callback returns
// an error.
//
// If a new device is observed, fn will be called with that device.
func ListenAndRegister(c context.Context, l *Listener, reg *Registry, fn func(d device.D) error) error {
	for {
		dh, err := l.Accept(c)
		if err != nil {
			// Note: may be a Context cancellation / deadline.
			return err
		}
		if d, isNew := reg.Observe(dh); isNew && fn != nil {
			if err := fn(d); err != nil {
				return err
			}
		}
	}
}
