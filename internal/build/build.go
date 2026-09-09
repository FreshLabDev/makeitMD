// SPDX-License-Identifier: Apache-2.0

// Package build carries what the linker stamped into the binary.
//
// One value feeds both `/healthz` and the About screen, so an operator reading
// the health endpoint and a user reading the panel can never be told two
// different versions of the same running process.
package build

// Info is the build identity: what `BUILD_VERSION`, `BUILD_COMMIT` and
// `BUILD_DATE` were at link time.
type Info struct {
	Version string
	Commit  string
	Date    string
}
