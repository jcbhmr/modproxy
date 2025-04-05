package proxy

import "time"

// TODO: Add more fields?
// https://pkg.go.dev/cmd/go/internal/modfetch#RevInfo
// https://pkg.go.dev/cmd/go/internal/modfetch/codehost#RevInfo

// A RevInfo describes a single revision in a module repository.
type RevInfo struct {
	Version string    // version string
	Time    time.Time `json:",omitzero"` // time of the version

	// These fields are used for Stat of arbitrary rev,
	// but they are not recorded when talking about module versions.
	Name  string `json:"-"` // complete ID in underlying repository
	Short string `json:"-"` // shortened ID, for use in pseudo-version

	Origin *Origin `json:",omitempty"`
}

type Origin struct {
	VCS    string `json:",omitempty"` // "git" etc
	URL    string `json:",omitempty"` // URL of repository
	Subdir string `json:",omitempty"` // subdirectory in repo

	Hash string `json:",omitempty"` // commit hash or ID

	// If TagSum is non-empty, then the resolution of this module version
	// depends on the set of tags present in the repo, specifically the tags
	// of the form TagPrefix + a valid semver version.
	// If the matching repo tags and their commit hashes still hash to TagSum,
	// the Origin is still valid (at least as far as the tags are concerned).
	// The exact checksum is up to the Repo implementation; see (*gitRepo).Tags.
	TagPrefix string `json:",omitempty"`
	TagSum    string `json:",omitempty"`

	// If Ref is non-empty, then the resolution of this module version
	// depends on Ref resolving to the revision identified by Hash.
	// If Ref still resolves to Hash, the Origin is still valid (at least as far as Ref is concerned).
	// For Git, the Ref is a full ref like "refs/heads/main" or "refs/tags/v1.2.3",
	// and the Hash is the Git object hash the ref maps to.
	// Other VCS might choose differently, but the idea is that Ref is the name
	// with a mutable meaning while Hash is a name with an immutable meaning.
	Ref string `json:",omitempty"`

	// If RepoSum is non-empty, then the resolution of this module version
	// failed due to the repo being available but the version not being present.
	// This depends on the entire state of the repo, which RepoSum summarizes.
	// For Git, this is a hash of all the refs and their hashes.
	RepoSum string `json:",omitempty"`
}
