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

func TestFilletedFrustumNearestMeridianArc(t *testing.T) {
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
	for end, c := range q.fillets.Circles {
		p := [3]*big.Rat{fdMul(c.Center[0], r(3, 5)), fdMul(c.Center[0], r(4, 5)), c.Center[1]}
		got, e := q.BoundToroidCenterNearestArcRational(ctx, p, opts)
		if e != nil || got.Arc == nil {
			t.Fatal(end, got, e)
		}
		a := got.Arc
		if a.Face != q.fillets.Blends[end] || a.SquaredDistance.Cmp(fdSquare(c.Radius)) != 0 {
			t.Fatal("original blend distance")
		}
		middle := [2]*big.Rat{r(5, 13), r(-12, 13)}
		if end == 1 {
			middle = [2]*big.Rat{r(3, 5), r(4, 5)}
		}
		for k, n := range [][2]*big.Rat{a.EndNormals[0], middle, a.EndNormals[1]} {
			var point [3]*big.Rat
			for i := range point {
				point[i] = fdAdd(a.Center[i], fdMul(a.Radius, fdAdd(fdMul(a.Radial[i], n[0]), fdMul(a.Axis[i], n[1]))))
			}
			state, e := q.ClassifyRational(ctx, point)
			if e != nil || state.Location != geom.BoreBoundary {
				t.Fatal("retained arc", state, e)
			}
			owners := map[int]bool{}
			for _, f := range state.Faces {
				owners[f] = true
			}
			if !owners[a.Face] {
				t.Fatal("blend owner")
			}
			if k == 1 {
				if len(owners) != 1 {
					t.Fatal("interior owner")
				}
			} else {
				idx := 0
				if k == 2 {
					idx = 1
				}
				if len(owners) != 2 || !owners[a.EndpointFaces[idx][1]] || len(a.RimEdges[idx]) != 4 {
					t.Fatal("paired endpoint ownership")
				}
			}
			d := new(big.Rat)
			for i := range p {
				d.Add(d, fdSquare(fdSub(point[i], p[i])))
			}
			if d.Cmp(fdSquare(c.Radius)) != 0 {
				t.Fatal("analytic equal distance")
			}
		}
		placedResult, e := placed.BoundToroidCenterNearestArcRational(ctx, world(p), opts)
		if e != nil || placedResult.WorldArc == nil {
			t.Fatal(e)
		}
		wa := placedResult.WorldArc
		for i := 0; i < 3; i++ {
			if wa.Center[i].Cmp(world(p)[i]) != 0 || wa.Axis[i].Cmp(m[i][2]) != 0 || wa.Radial[i].Cmp(fdAdd(fdMul(m[i][0], r(3, 5)), fdMul(m[i][1], r(4, 5)))) != 0 {
				t.Fatal("world arc basis")
			}
		}
		if wa.Face != a.Face || wa.SquaredDistance.Cmp(a.SquaredDistance) != 0 || wa.EndpointFaces != a.EndpointFaces {
			t.Fatal("world provenance")
		}
		capQuery := p
		capQuery[2] = q.z[end]
		rejected, e := q.BoundToroidCenterNearestArcRational(ctx, capQuery, opts)
		if e != nil || rejected.Arc != nil || rejected.Baseline.Upper.Sign() != 0 {
			t.Fatal("closer adjacent cap", e)
		}
		a.RimEdges[0][0] = -999
		a.Radius.SetInt64(999)
		fresh, e := q.BoundToroidCenterNearestArcRational(ctx, p, opts)
		if e != nil || fresh.Arc == nil || fresh.Arc.RimEdges[0][0] < 0 || fresh.Arc.Radius.Cmp(c.Radius) != 0 {
			t.Fatal("owned arc", e)
		}
	}
	axis, e := q.BoundToroidCenterNearestArcRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(3, 1)}, opts)
	if e != nil || axis.Arc != nil {
		t.Fatal("no stationary axis maximum", e)
	}
	bad, e := q.BoundToroidCenterNearestArcRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(3, 1)}, FilletedFrustumDistanceOptions{MaxSupportChecks: 1})
	if e == nil || bad.Arc != nil || bad.Baseline.Lower != nil {
		t.Fatal("atomic budget")
	}
}
