package authz

import "testing"

func TestParsePermissions_Full(t *testing.T) {
	p := ParsePermissions("rwd|r--|---")

	// Owner is always rwd.
	if !p.Owner.CanRead() || !p.Owner.CanWrite() || !p.Owner.CanDelete() {
		t.Error("owner should have full permissions")
	}
	// Group has read only.
	if !p.Group.CanRead() {
		t.Error("group should have read")
	}
	if p.Group.CanWrite() {
		t.Error("group should not have write")
	}
	if p.Group.CanDelete() {
		t.Error("group should not have delete")
	}
	// Others have nothing.
	if p.Others.CanRead() || p.Others.CanWrite() || p.Others.CanDelete() {
		t.Error("others should have no permissions")
	}
}

func TestParsePermissions_DefaultPrivate(t *testing.T) {
	p := ParsePermissions("rwd|---|---")
	if !p.Owner.CanRead() || !p.Owner.CanWrite() || !p.Owner.CanDelete() {
		t.Error("owner should have full permissions")
	}
	if p.Group.CanRead() || p.Group.CanWrite() || p.Group.CanDelete() {
		t.Error("group should have no permissions")
	}
	if p.Others.CanRead() || p.Others.CanWrite() || p.Others.CanDelete() {
		t.Error("others should have no permissions")
	}
}

func TestParsePermissions_InvalidString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"garbage", "xyz"},
		{"single segment", "rwd"},
		{"two segments", "rwd|r--"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePermissions(tt.input)
			// Owner is always forced to rwd.
			if !p.Owner.CanRead() || !p.Owner.CanWrite() || !p.Owner.CanDelete() {
				t.Error("owner should always be rwd")
			}
		})
	}
}

func TestParsePermissions_OwnerForcedToRwd(t *testing.T) {
	// Even if input has restricted owner, it should be forced to rwd.
	p := ParsePermissions("r--|r--|---")
	if !p.Owner.CanRead() || !p.Owner.CanWrite() || !p.Owner.CanDelete() {
		t.Error("owner should be forced to rwd regardless of input")
	}
}

func TestPermissions_String_Roundtrip(t *testing.T) {
	tests := []string{
		"rwd|r--|---",
		"rwd|---|---",
		"rwd|rwd|r--",
		"rwd|rw-|r--",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			p := ParsePermissions(input)
			output := p.String()
			if output != input {
				t.Errorf("roundtrip failed: input=%q output=%q", input, output)
			}
		})
	}
}

func TestPermSet_CanReadWriteDelete(t *testing.T) {
	ps := PermSet{Read: true, Write: false, Delete: true}
	if !ps.CanRead() {
		t.Error("expected CanRead true")
	}
	if ps.CanWrite() {
		t.Error("expected CanWrite false")
	}
	if !ps.CanDelete() {
		t.Error("expected CanDelete true")
	}
}

func TestDefaultPermissions(t *testing.T) {
	p := DefaultPermissions()
	if p.String() != "rwd|---|---" {
		t.Errorf("expected rwd|---|---, got %s", p.String())
	}
}
