// Package authz provides resource-level authorization primitives.
package authz

import "strings"

// PermSet represents read, write, delete permissions for a single level.
type PermSet struct {
	Read   bool
	Write  bool
	Delete bool
}

// CanRead returns true if read permission is granted.
func (p PermSet) CanRead() bool { return p.Read }

// CanWrite returns true if write permission is granted.
func (p PermSet) CanWrite() bool { return p.Write }

// CanDelete returns true if delete permission is granted.
func (p PermSet) CanDelete() bool { return p.Delete }

// String renders the PermSet as a 3-character string (e.g., "rw-", "r--", "---").
func (p PermSet) String() string {
	var b [3]byte
	if p.Read {
		b[0] = 'r'
	} else {
		b[0] = '-'
	}
	if p.Write {
		b[1] = 'w'
	} else {
		b[1] = '-'
	}
	if p.Delete {
		b[2] = 'd'
	} else {
		b[2] = '-'
	}
	return string(b[:])
}

// Permissions represents owner/group/others permission levels for a resource.
type Permissions struct {
	Owner  PermSet
	Group  PermSet
	Others PermSet
}

// DefaultPermissions returns the default private permissions: "rwd|---|---".
func DefaultPermissions() Permissions {
	return Permissions{
		Owner: PermSet{Read: true, Write: true, Delete: true},
	}
}

// ParsePermissions parses a permission string in "rwd|r--|---" format.
// Each segment is 3 characters: r=Read, w=Write, d=Delete, -=denied.
// Invalid or missing segments default to no permissions.
// Owner level is always forced to "rwd" regardless of input.
func ParsePermissions(s string) Permissions {
	parts := strings.Split(s, "|")

	p := Permissions{
		// Owner is always rwd (immutable).
		Owner: PermSet{Read: true, Write: true, Delete: true},
	}

	if len(parts) >= 2 {
		p.Group = parsePermSet(parts[1])
	}
	if len(parts) >= 3 {
		p.Others = parsePermSet(parts[2])
	}

	return p
}

// String renders permissions as "rwd|r--|---" format.
func (p Permissions) String() string {
	return p.Owner.String() + "|" + p.Group.String() + "|" + p.Others.String()
}

// parsePermSet parses a 3-character permission segment.
func parsePermSet(s string) PermSet {
	if len(s) != 3 {
		return PermSet{}
	}
	return PermSet{
		Read:   s[0] == 'r',
		Write:  s[1] == 'w',
		Delete: s[2] == 'd',
	}
}
