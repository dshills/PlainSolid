package parametric

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestFiniteObliqueParallelCylinderSets(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{X: 3, Y: 4}, Radius: 5, Min: 0, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 6, Y: 8, Z: 5}, Axis: geom.Vec3{X: -6, Y: -8}, Radius: 5, Min: 0, Max: 1}
	common := 10 * (50*math.Pi/3 - 25*math.Sqrt(3)/2)
	volume := 250 * math.Pi
	for _, swap := range []bool{false, true} {
		s, tool := a, b
		if swap {
			s, tool = b, a
		}
		u, err := FiniteCylinderPairUnion(ctx, s, tool, 1e-9)
		if err != nil || len(u.Solid.Faces()) != 6 || len(u.WallFaces[0]) != 1 || len(u.WallFaces[1]) != 1 {
			t.Fatal("union ancestry", err)
		}
		if u.CommonVolumeSquared != nil || u.CommonVolumePiCoefficient != nil {
			t.Fatal("transverse common does not have a rational volume proof")
		}
		for i, c := range [2]geom.RawCylinder{s, tool} {
			for j, ids := range [2][]int{u.LowerCapFaces[i], u.UpperCapFaces[i]} {
				if len(ids) != 1 {
					t.Fatal("cap ancestry", i, j, ids)
				}
				face := u.Solid.Faces()[ids[0]]
				p, err := face.Surface.Eval(face.U.Min, face.V.Max)
				origin := c.Origin.Add(c.Axis.Scale([2]float64{c.Min, c.Max}[j]))
				if err != nil || math.Abs(p.Sub(origin).Dot(c.Axis)) > 1e-9 {
					t.Fatal("signed cap plane", i, j, p, err)
				}
			}
		}
		d, err := FiniteCylinderPairDifference(ctx, s, tool, 1e-9)
		if err != nil || len(d.Solids[0].Faces()) != 4 || len(d.Solids[1].Faces()) != 0 || len(d.StockCapFaces[0]) != 2 || len(d.CutterWallFaces[0]) != 1 {
			t.Fatal("difference ancestry", err)
		}
		for i, solid := range []Solid{u.Solid, d.Solids[0]} {
			m, err := solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
			want := [2]float64{2*volume - common, volume - common}[i]
			if err != nil || math.Abs(m.Volume-want) > 1e-7 || math.Abs(m.Centroid.X-3) > 1e-9 || math.Abs(m.Centroid.Y-4) > 1e-9 || i == 0 && math.Abs(m.Centroid.Z-2.5) > 1e-9 {
				t.Fatal("analytic volume and center", i, m, err)
			}
			mesh, err := solid.Tessellate(ctx, TessellationOptions{Deflection: .1})
			if err != nil || mesh.ValidateSolid() != nil {
				t.Fatal("closed display mesh", i, err)
			}
			var buf bytes.Buffer
			if err := WriteJSON(&buf, solid); err != nil {
				t.Fatal(err)
			}
			reload, err := ReadJSON(&buf, JSONLimits{})
			if err != nil || len(reload.Faces()) != len(solid.Faces()) {
				t.Fatal("native reload", err)
			}
		}
	}
	for _, bad := range []geom.RawCylinder{
		{Origin: b.Origin, Axis: b.Axis, Radius: 5, Min: 0, Max: .75},                   // different caps
		{Origin: geom.Vec3{X: 6, Y: 8, Z: 10}, Axis: b.Axis, Radius: 5, Min: 0, Max: 1}, // tangent
	} {
		if r, err := FiniteCylinderPairUnion(ctx, a, bad, 1e-9); !errors.Is(err, geom.ErrDomain) || len(r.Solid.Faces()) != 0 {
			t.Fatal("unqualified set accepted", err)
		}
	}
}
