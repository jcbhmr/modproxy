package modproxy

import "time"

type Info struct {
	Version string    // version string
	Time    time.Time `json:",omitzero"` // time of the version
}

// TODO: Add more fields
// https://pkg.go.dev/cmd/go/internal/modfetch#RevInfo
// https://pkg.go.dev/cmd/go/internal/modfetch/codehost#RevInfo
