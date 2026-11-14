package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumNearestCircle(t *testing.T) {
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
	for ci, c := range q.cuts {
		z := new(big.Rat).Quo(fdAdd(c.z[0], c.z[1]), r(2, 1))
		p := [3]*big.Rat{c.center[0], c.center[1], z}
		got, e := q.BoundBoreAxisNearestCircleRational(ctx, p, opts)
		if e != nil || got.Circle == nil || !got.Baseline.Exact {
			t.Fatal(ci, got, e)
		}
		circle := got.Circle
		if circle.Face != c.wall || circle.SquaredDistance.Cmp(r(1, 4)) != 0 {
			t.Fatal("owner/distance")
		}
		// Distinct exact rational azimuths independently retain the original wall
		// and equal distance, including points on opposite native seam quadrants.
		for _, uv := range [][2]*big.Rat{{r(1, 1), r(0, 1)}, {r(3, 5), r(4, 5)}, {r(-5, 13), r(-12, 13)}} {
			sample := [3]*big.Rat{fdAdd(p[0], fdMul(c.radius, uv[0])), fdAdd(p[1], fdMul(c.radius, uv[1])), new(big.Rat).Set(z)}
			state, e := q.ClassifyRational(ctx, sample)
			if e != nil || state.Location != geom.BoreBoundary || len(state.Faces) != 1 || state.Faces[0] != c.wall {
				t.Fatal("original circle retention", state, e)
			}
			d := fdAdd(fdSquare(fdSub(sample[0], p[0])), fdSquare(fdSub(sample[1], p[1])))
			if d.Cmp(circle.SquaredDistance) != 0 {
				t.Fatal("same distance")
			}
		}
		posed, e := placed.BoundBoreAxisNearestCircleRational(ctx, world(p), opts)
		if e != nil || posed.WorldCircle == nil {
			t.Fatal(e)
		}
		wc := posed.WorldCircle
		dot := func(a, b [3]*big.Rat) *big.Rat {
			x := new(big.Rat)
			for i := range a {
				x.Add(x, fdMul(a[i], b[i]))
			}
			return x
		}
		if dot(wc.U, wc.U).Cmp(r(1, 1)) != 0 || dot(wc.V, wc.V).Cmp(r(1, 1)) != 0 || dot(wc.U, wc.V).Sign() != 0 || wc.Face != circle.Face || wc.SquaredDistance.Cmp(circle.SquaredDistance) != 0 {
			t.Fatal("world continuum metric")
		}
		for i := 0; i < 3; i++ {
			if wc.Center[i].Cmp(world(p)[i]) != 0 || wc.U[i].Cmp(m[i][0]) != 0 || wc.V[i].Cmp(m[i][1]) != 0 {
				t.Fatal("world basis")
			}
		}
		circle.Center[0].SetInt64(999)
		circle.Radius.SetInt64(999)
		fresh, e := q.BoundBoreAxisNearestCircleRational(ctx, p, opts)
		if e != nil || fresh.Circle == nil || fresh.Circle.Radius.Cmp(r(1, 2)) != 0 {
			t.Fatal("owned result", e)
		}
	}
	// Lower blind midpoint ties with floor, so circle is a subset, not an
	// assertion that no additional nearest contact exists.
	tie, e := q.BoundBoreAxisNearestCircleRational(ctx, [3]*big.Rat{r(2, 1), r(0, 1), r(1, 2)}, opts)
	if e != nil || tie.Circle == nil {
		t.Fatal("permitted tie", e)
	}
	for _, p := range [][3]*big.Rat{{r(2, 1), r(0, 1), r(9, 10)}, {r(2, 1), r(0, 1), r(1, 1)}, {r(-19, 10), r(0, 1), r(3, 1)}} {
		got, e := q.BoundBoreAxisNearestCircleRational(ctx, p, opts)
		if e != nil || got.Circle != nil {
			t.Fatal("closer floor/end/off-axis", got, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := placed.BoundBoreAxisNearestCircleRational(cancelled, world([3]*big.Rat{r(-2, 1), r(0, 1), r(3, 1)}), opts)
	if e == nil || bad.WorldCircle != nil || bad.Original.Baseline.Lower != nil {
		t.Fatal("atomic cancellation")
	}
	failed, e := q.BoundBoreAxisNearestCircleRational(ctx, [3]*big.Rat{r(-2, 1), r(0, 1), r(3, 1)}, FilletedFrustumDistanceOptions{MaxSupportChecks: 1})
	if e == nil || failed.Circle != nil || failed.Baseline.Lower != nil {
		t.Fatal("atomic budget")
	}
}
