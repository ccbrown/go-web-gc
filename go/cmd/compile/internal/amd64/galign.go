// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package amd64

import (
	"github.com/ccbrown/go-web-gc/go/cmd/compile/internal/gc"
	"github.com/ccbrown/go-web-gc/go/cmd/internal/obj/x86"
	"github.com/ccbrown/go-web-gc/go/cmd/internal/objabi"
)

var leaptr = x86.ALEAQ

func Init(arch *gc.Arch) {
	arch.LinkArch = &x86.Linkamd64
	if objabi.GOARCH == "amd64p32" {
		arch.LinkArch = &x86.Linkamd64p32
		leaptr = x86.ALEAL
	}
	arch.REGSP = x86.REGSP
	arch.MAXWIDTH = 1 << 50

	arch.ZeroRange = zerorange
	arch.ZeroAuto = zeroAuto
	arch.Ginsnop = ginsnop
	arch.Ginsnopdefer = ginsnop

	arch.SSAMarkMoves = ssaMarkMoves
	arch.SSAGenValue = ssaGenValue
	arch.SSAGenBlock = ssaGenBlock
}
