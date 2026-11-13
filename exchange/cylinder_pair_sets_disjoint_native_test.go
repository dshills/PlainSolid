package exchange

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCylinderPairUnionDisjointNative(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Radius: 2, Min: -1, Max: 1}
	for _, origin := range []geom.Vec3{{X: 4}, {Z: 8}} {
		r := CylinderPairCommonRecipe{Name: "independent shells", A: a, B: geom.RawCylinder{Origin: origin, Axis: geom.Vec3{Z: -3}, Radius: 1, Min: -1, Max: 1}, Tolerance: 1e-9, LengthUnit: .001}
		var b bytes.Buffer
		if e := WriteCylinderPairUnionRecipeJSON(ctx, &b, r); e != nil {
			t.Fatal(e)
		}
		got, u, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(b.Bytes()), 0)
		if e != nil || got != r || u.CommonVolumeSquared == nil || u.CommonVolumeSquared.Sign() != 0 {
			t.Fatal(got, u.CommonVolumeSquared, e)
		}
		faces := u.Solid.Faces()
		if len(faces) != 6 {
			t.Fatal("faces", len(faces))
		}
		seen := map[int]bool{}
		for owner, raw := range []geom.RawCylinder{a, r.B} {
			for role, ids := range [][]int{u.WallFaces[owner], u.LowerCapFaces[owner], u.UpperCapFaces[owner]} {
				if len(ids) != 1 {
					t.Fatal("lost operand role")
				}
				id := ids[0]
				if seen[id] {
					t.Fatal("merged ancestry")
				}
				seen[id] = true
				f := faces[id]
				if role > 0 {
					p, e := f.Surface.Eval(0, 0)
					want := raw.Min
					if role == 2 {
						want = raw.Max
					}
					if e != nil || p.Sub(raw.Origin).Dot(raw.Axis)/raw.Axis.Dot(raw.Axis) != want {
						t.Fatal("raw cap changed", p, e)
					}
				} else {
					p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2)
					d := p.Sub(raw.Origin)
					q := d.Dot(d) - math.Pow(d.Dot(raw.Axis), 2)/raw.Axis.Dot(raw.Axis)
					if e != nil || math.Abs(q-raw.Radius*raw.Radius) > 1e-12 {
						t.Fatal("wall changed", q, e)
					}
				}
			}
		}
		// Count face components through shared topological edges, without meshing.
		components := func(s parametric.Solid) int {
			fs := s.Faces()
			parent := make([]int, len(fs))
			for i := range parent {
				parent[i] = i
			}
			var root func(int) int
			root = func(i int) int {
				if parent[i] != i {
					parent[i] = root(parent[i])
				}
				return parent[i]
			}
			owner := map[int]int{}
			for i, f := range fs {
				for _, use := range f.Boundary {
					if j, ok := owner[use.Edge]; ok {
						parent[root(i)] = root(j)
					} else {
						owner[use.Edge] = i
					}
				}
			}
			n := 0
			for i := range parent {
				if root(i) == i {
					n++
				}
			}
			return n
		}
		if components(u.Solid) != 2 {
			t.Fatal("shells flattened")
		}
		var native bytes.Buffer
		if e = parametric.WriteJSON(&native, u.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := parametric.ReadJSON(bytes.NewReader(native.Bytes()), parametric.JSONLimits{})
		if e != nil || components(loaded) != 2 {
			t.Fatal("native shell loss", e)
		}
		var restored bytes.Buffer
		if e = parametric.WriteJSON(&restored, loaded); e != nil || !bytes.Equal(native.Bytes(), restored.Bytes()) {
			t.Fatal("native chart changed", e)
		}
		var again bytes.Buffer
		if e = WriteCylinderPairUnionRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
			t.Fatal("recipe changed", e)
		}
	}
}

func TestCylinderPairContainedUnionRecipe(t *testing.T) {
	ctx := context.Background()
	r := CylinderPairCommonRecipe{
		Name: "contained skew", LengthUnit: .001, Tolerance: 1e-9,
		A: geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 2}, Radius: 2, Min: -2, Max: 2},
		B: geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 4, Z: 4}, Axis: geom.Vec3{Z: -4}, Radius: 5, Min: -.5, Max: .5},
	}
	var buffer bytes.Buffer
	if e := WriteCylinderPairUnionRecipeJSON(ctx, &buffer, r); e != nil {
		t.Fatal(e)
	}
	p, u, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(buffer.Bytes()), 0)
	if e != nil || p != r || u.CommonVolumeSquared != nil || u.CommonVolumePiCoefficient == nil || u.CommonVolumePiCoefficient.RatString() != "32" || len(u.Solid.Faces()) != 3 {
		t.Fatal(p, u, e)
	}
	bad := bytes.Replace(buffer.Bytes(), []byte(`"CommonVolumePiCoefficient":"32"`), []byte(`"CommonVolumePiCoefficient":"31"`), 1)
	if bytes.Equal(buffer.Bytes(), bad) {
		t.Fatal("fixture did not carry containment proof")
	}
	if _, u, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(bad), 0); e == nil || len(u.Solid.Faces()) != 0 {
		t.Fatal("accepted altered containment proof", e)
	}
}
