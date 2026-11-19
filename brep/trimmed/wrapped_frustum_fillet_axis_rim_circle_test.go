//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestFilletedFrustumNearestEntryCircle(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	r := big.NewRat
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &encoded, q); e != nil {
		t.Fatal(e)
	}
	q, e = ReadFilletedFrustumMaterialJSON(ctx, &encoded, FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m := [3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	placed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		var v [3]*big.Rat
		for i := range v {
			v[i] = new(big.Rat).Set(m[i][3])
			for j := range p {
				v[i].Add(v[i], fdMul(m[i][j], p[j]))
			}
		}
		return v
	}

	opts := FilletedFrustumDistanceOptions{}
	for _, c := range q.cuts {
		for end, z := range c.z {
			if !c.open[end] {
				continue
			}
			height := fdSub(z, r(1, 8))
			if end == 1 {
				height = fdAdd(z, r(1, 8))
			}
			p := [3]*big.Rat{c.center[0], c.center[1], height}
			got, e := q.BoundEntryRimAxisNearestCircleRational(ctx, p, opts)
			if e != nil || got.Rim == nil || !got.Baseline.Exact {
				t.Fatal(got, e)
			}
			rim := got.Rim
			if rim.Circle.SquaredDistance.Cmp(r(17, 64)) != 0 || rim.Circle.Face != c.wall || len(rim.Faces) != 2 || len(rim.RimEdges) != 4 {
				t.Fatal("circle ancestry/distance")
			}
			for _, uv := range [][2]*big.Rat{{r(1, 1), r(0, 1)}, {r(3, 5), r(4, 5)}, {r(-5, 13), r(-12, 13)}} {
				point := [3]*big.Rat{fdAdd(c.center[0], fdMul(c.radius, uv[0])), fdAdd(c.center[1], fdMul(c.radius, uv[1])), z}
				state, e := q.ClassifyRational(ctx, point)
				if e != nil || state.Location != geom.BoreBoundary || len(state.Faces) != 2 {
					t.Fatal("full retained rim", state, e)
				}
				for i, f := range state.Faces {
					if f != rim.Faces[i] {
						t.Fatal("owner mismatch")
					}
				}
				d := new(big.Rat)
				for i := range point {
					d.Add(d, fdSquare(fdSub(point[i], p[i])))
				}
				if d.Cmp(r(17, 64)) != 0 {
					t.Fatal("independent Pythagorean distance")
				}
			}
			placedResult, e := placed.BoundEntryRimAxisNearestCircleRational(ctx, world(p), opts)
			if e != nil || placedResult.WorldRim == nil {
				t.Fatal(e)
			}
			wc := placedResult.WorldRim.Circle
			for i := 0; i < 3; i++ {
				if wc.Center[i].Cmp(world(rim.Circle.Center)[i]) != 0 || wc.U[i].Cmp(m[i][0]) != 0 || wc.V[i].Cmp(m[i][1]) != 0 {
					t.Fatal("world exact circle")
				}
			}
			if wc.SquaredDistance.Cmp(r(17, 64)) != 0 || wc.Face != c.wall {
				t.Fatal("world owner/distance")
			}
			rim.Faces[0] = -1
			rim.RimEdges[0] = -1
			rim.Circle.Radius.SetInt64(999)
			fresh, e := q.BoundEntryRimAxisNearestCircleRational(ctx, p, opts)
			if e != nil || fresh.Rim == nil || fresh.Rim.Faces[0] < 0 || fresh.Rim.RimEdges[0] < 0 || fresh.Rim.Circle.Radius.Cmp(r(1, 2)) != 0 {
				t.Fatal("result ownership", e)
			}
		}
	}
	for _, p := range [][3]*big.Rat{{r(-2, 1), r(0, 1), r(3, 1)}, {r(2, 1), r(0, 1), r(9, 10)}, {r(-19, 10), r(0, 1), r(-1, 8)}} {
		got, e := q.BoundEntryRimAxisNearestCircleRational(ctx, p, opts)
		if e != nil || got.Rim != nil {
			t.Fatal("interior/floor/off-axis rejection", e)
		}
	}
	bad, e := q.BoundEntryRimAxisNearestCircleRational(ctx, [3]*big.Rat{r(-2, 1), r(0, 1), r(-1, 8)}, FilletedFrustumDistanceOptions{MaxSupportChecks: 1})
	if e == nil || bad.Rim != nil || bad.Baseline.Lower != nil {
		t.Fatal("atomic budget")
	}
}
