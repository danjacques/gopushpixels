// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPixelPusher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PixelPusher Tests")
}
