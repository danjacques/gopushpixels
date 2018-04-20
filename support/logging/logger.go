// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package logging

// L accepts logging data.
//
// L is designed to automatically conform to zap's zap.SugaredLogger, but is
// generic enough that any logger should be able to match it.
type L interface {
	// Error emits an error-level log.
	Error(args ...interface{})
	// Warn emits an error-level log.
	Warn(args ...interface{})
	// info emits an error-level log.
	Info(args ...interface{})
	// Debug emits an error-level log.
	Debug(args ...interface{})

	// Errorf emits an error-level log.
	Errorf(fmt string, args ...interface{})
	// Warnf emits an error-level log.
	Warnf(fmt string, args ...interface{})
	// infof emits an error-level log.
	Infof(fmt string, args ...interface{})
	// Debugf emits an error-level log.
	Debugf(fmt string, args ...interface{})
}

// Nop is a L instance that does nothing.
var Nop L = nopLogger{}

// Must ensures that a valid L is available. If l is not nil, it will be
// returned; otherwise, Must will return Nop.
func Must(l L) L {
	if l != nil {
		return l
	}
	return Nop
}

type nopLogger struct{}

func (nopLogger) Error(args ...interface{}) {}
func (nopLogger) Warn(args ...interface{})  {}
func (nopLogger) Info(args ...interface{})  {}
func (nopLogger) Debug(args ...interface{}) {}

func (nopLogger) Errorf(fmt string, args ...interface{}) {}
func (nopLogger) Warnf(fmt string, args ...interface{})  {}
func (nopLogger) Infof(fmt string, args ...interface{})  {}
func (nopLogger) Debugf(fmt string, args ...interface{}) {}
