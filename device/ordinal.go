// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"fmt"
)

// Ordinal is the device's ordinal value, identifying which logical group it
// belongs to.
type Ordinal struct {
	Group      int
	Controller int
}

func (o *Ordinal) String() string {
	if !o.IsValid() {
		return "{INVALID}"
	}
	return fmt.Sprintf("{Grp=%d, Cont=%d}", o.Group, o.Controller)
}

// InvalidOrdinal returns an Ordinal that registers as invalid.
func InvalidOrdinal() Ordinal {
	return Ordinal{Group: -1, Controller: -1}
}

// IsValid returns true if this is a valid Ordinal. An ordinal is valid if both
// its Group and Controller are >0.
func (o *Ordinal) IsValid() bool {
	return o.Group >= 0 && o.Controller >= 0
}
