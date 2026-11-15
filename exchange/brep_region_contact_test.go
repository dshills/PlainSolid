package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func nativePlanarRegions(t *testing.T) [2]brep.Solid {
	t.Helper()
	unit := geom.Vec3{X: 1, Y: 1, Z: 1}
	base, err := brep.Box(geom.Vec3{}, unit)
	if err != nil {
		t.Fatal(err)
	}
	v, e, f := base.Vertices(), base.Edges(), base.Faces()
	wire := f[1].Boundary
	start := func(u brep.EdgeUse) int {
		if u.Reversed {
			return e[u.Edge].End
		}
		return e[u.Edge].Start
	}
	d := len(e)
	e = append(e, brep.Edge{Start: start(wire[0]), End: start(wire[2])})
	f[1].Boundary = []brep.EdgeUse{wire[0], wire[1], {Edge: d, Reversed: true}}
	f = append(f, brep.Face{Boundary: []brep.EdgeUse{{Edge: d}, wire[2], wire[3]}})
	lower, err := brep.NewSolid(v, e, f)
	if err != nil {
		t.Fatal(err)
	}
	z := geom.Vec3{Z: 1}
	upper, err := brep.Box(z, z.Add(unit))
	if err != nil {
		t.Fatal(err)
	}
	return [2]brep.Solid{lower, upper}
}

func TestPlanarRegionContactNativeReceipt(t *testing.T) {
	ctx := context.Background()
	parts := nativePlanarRegions(t)
	o := PlanarRegionContactJSONOptions{}
	var buf bytes.Buffer
	if err := WritePlanarRegionContactPairJSON(ctx, &buf, parts, o); err != nil {
		t.Fatal(err)
	}
	raw := bytes.Clone(buf.Bytes())
	got, p, err := ReadPlanarRegionContactPairJSON(ctx, bytes.NewReader(raw), o)
	if err != nil || !reflect.DeepEqual(got, parts) || !reflect.DeepEqual(p.Faces, [2][]int{{1, 6}, {0}}) ||
		p.SignedVolume.RatString() != "2" || len(p.Vertices) != 4 || len(p.Edges) != 4 {
		t.Fatalf("native reload changed an original body or face: %+v %v", p, err)
	}
	buf.Reset()
	if err := WritePlanarRegionContactPairJSON(ctx, &buf, got, o); err != nil || !bytes.Equal(raw, buf.Bytes()) {
		t.Fatal("noncanonical native replay", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*planarRegionContactDocument)
	}{
		{"lost-face", func(d *planarRegionContactDocument) { d.Witness.Faces[0] = d.Witness.Faces[0][:1] }},
		{"extra-edge-owner", func(d *planarRegionContactDocument) { d.Witness.Edges[0] = append(d.Witness.Edges[0], 123) }},
		{"changed-projected-area", func(d *planarRegionContactDocument) { d.Witness.TwiceProjectedAreas[0][0] = "999/1000" }},
		{"wrong-vertex-pair", func(d *planarRegionContactDocument) { d.Witness.Vertices[0][1] = 99 }},
		{"wrong-projection", func(d *planarRegionContactDocument) { d.Witness.DropAxes[1][0] = 0 }},
		{"same-volume-not-canonical", func(d *planarRegionContactDocument) { d.Witness.Volume = "4/2" }},
		{"extra-component", func(d *planarRegionContactDocument) { d.Components = append(d.Components, d.Components[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var d planarRegionContactDocument
			if err := json.Unmarshal(raw, &d); err != nil {
				t.Fatal(err)
			}
			tc.edit(&d)
			bad, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			s, p, err := ReadPlanarRegionContactPairJSON(ctx, bytes.NewReader(bad), o)
			if !errors.Is(err, collision.ErrInvalid) || p.SignedVolume != nil || len(s[0].Vertices()) != 0 {
				t.Fatalf("invalid receipt published bodies: %+v %v", p, err)
			}
		})
	}
	buf.Reset()
	if err := WritePlanarRegionContactPairJSON(ctx, &buf, parts, PlanarRegionContactJSONOptions{MaxBytes: 10}); !errors.Is(err, collision.ErrLimit) || buf.Len() != 0 {
		t.Fatalf("bounded writer published a partial source: %v", err)
	}
	if got, p, err := ReadPlanarRegionContactPairJSON(ctx, bytes.NewReader(raw), PlanarRegionContactJSONOptions{MaxBytes: 10}); !errors.Is(err, collision.ErrLimit) || p.SignedVolume != nil || len(got[0].Vertices()) != 0 {
		t.Fatalf("bounded reader published a partial source: %+v %v", p, err)
	}
}
