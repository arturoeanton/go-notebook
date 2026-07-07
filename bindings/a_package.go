// Package bindings registers precompiled gomacro import bindings for the
// libraries bundled with go-notebook, so notebook cells can import them
// instantly, without network access or runtime plugin compilation.
//
// The github_com_*.go files are generated with the gomacro CLI:
//
//	echo 'import _3 "github.com/arturoeanton/go-pandas"' | gomacro
//
// and then copied here changing the package clause to "bindings".
package bindings

import (
	. "reflect"
)

type Package = struct {
	Name    string
	Binds   map[string]Value
	Types   map[string]Type
	Proxies map[string]Type
	// Untypeds contains a string representation of untyped constants,
	// stored without loss of precision
	Untypeds map[string]string
	// Wrappers is the list of wrapper methods for named types.
	Wrappers map[string][]string
}

var Packages = make(map[string]Package)
