package collision

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func TestVertexContactAssemblyLocatorReturnsEveryPointOwner(t *testing.T) {
	ctx := context.Background()
	xyz := func(i float64) geom.Vec3 { return geom.Vec3{X: i, Y: i, Z: i} }
	a, err := brep.Box(xyz(0), xyz(1))
	if err != nil {
		t.Fatal(err)
	}
	b, err := brep.Box(xyz(1), xyz(2))
	if err != nil {
		t.Fatal(err)
	}
	parts := []brep.Solid{a, b}
	contact := assemblyContactAt(t, parts, 0, 1, xyz(1))
	declarations := []AssemblyVertexContact{contact}
	l, err := NewVertexContactAssemblyLocator(ctx, parts, declarations, VertexContactAssemblyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	declarations[0].A.Vertex = -1
	parts[0] = brep.Solid{}
	for _, tc := range []struct {
		p      geom.Vec3
		want   Location
		owners []int
	}{
		{xyz(-1), Outside, nil},
		{xyz(math.Nextafter(1, 0)), Inside, []int{0}},
		{xyz(1), OnBoundary, []int{0, 1}},
		{xyz(math.Nextafter(1, 2)), Inside, []int{1}},
		{geom.Vec3{X: .5, Y: .5, Z: 1}, OnBoundary, []int{0}},
		{geom.Vec3{X: .5, Y: 1.5, Z: 1}, Outside, nil},
	} {
		r, err := l.Locate(ctx, tc.p)
		if err != nil || r.Location != tc.want || len(r.Owners) != len(tc.owners) {
			t.Fatalf("point %v: %+v %v", tc.p, r, err)
		}
		for i, owner := range r.Owners {
			if owner.Solid != tc.owners[i] || (r.Location == OnBoundary) != (owner.Face >= 0 && owner.Triangle >= 0) {
				t.Fatalf("point %v lost source owner %+v", tc.p, owner)
			}
		}
		if tc.p == xyz(1) {
			if len(r.Contacts) != 1 || r.Contacts[0] != contact {
				t.Fatalf("point contact ledger changed: %+v", r.Contacts)
			}
			r.Contacts[0].A.Vertex = -1
		} else if len(r.Contacts) != 0 {
			t.Fatalf("noncontact query acquired a contact: %+v", r)
		}
	}
	report := l.Report()
	report.SignedVolume.SetInt64(999)
	report.Contacts[0].A.Vertex = 888
	if p := l.Report(); p.SignedVolume.Cmp(big.NewRat(2, 1)) != 0 || p.Contacts[0] != contact {
		t.Fatal("prepared proof aliases caller-owned diagnostics")
	}
	limited, err := NewVertexContactAssemblyLocator(ctx, []brep.Solid{a, b}, []AssemblyVertexContact{contact}, VertexContactAssemblyOptions{MaxQueryWork: 1})
	if err != nil {
		t.Fatal(err)
	}
	if r, err := limited.Locate(ctx, xyz(1)); !errors.Is(err, ErrLimit) || r.Owners != nil || r.Contacts != nil {
		t.Fatalf("budget failure published a partial query: %+v %v", r, err)
	}
}
