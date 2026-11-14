package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumConeNearestCircle(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	r := big.NewRat
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, .5, 6, o)
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
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./128, 15./64, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: 1. / 8}, Radius: 1. / 32, Depth: 1. / 8}}, o)
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

	p := [3]*big.Rat{r(0, 1), r(0, 1), r(3, 1)}
	got, e := q.BoundConeAxisNearestCircleRational(ctx, p, FilletedFrustumDistanceOptions{})
	if e != nil || got.Circle == nil {
		t.Fatal(got, e)
	}
	c := got.Circle
	if c.Radius.Cmp(r(81, 50)) != 0 || c.Center[2].Cmp(r(21, 25)) != 0 || c.SquaredDistance.Cmp(r(729, 100)) != 0 || c.Face != q.fillets.Lateral {
		t.Fatal("independent meridian foot", c)
	}
	for _, uv := range [][2]*big.Rat{{r(1, 1), r(0, 1)}, {r(3, 5), r(4, 5)}, {r(-5, 13), r(-12, 13)}} {
		sample := [3]*big.Rat{fdMul(c.Radius, uv[0]), fdMul(c.Radius, uv[1]), c.Center[2]}
		state, e := q.ClassifyRational(ctx, sample)
		if e != nil || state.Location != geom.BoreBoundary || len(state.Faces) != 1 || state.Faces[0] != c.Face {
			t.Fatal("retained cone", state, e)
		}
		d := new(big.Rat)
		for i := range p {
			d.Add(d, fdSquare(fdSub(sample[i], p[i])))
		}
		if d.Cmp(r(729, 100)) != 0 {
			t.Fatal("exact equal distance")
		}
	}
	posed, e := placed.BoundConeAxisNearestCircleRational(ctx, world(p), FilletedFrustumDistanceOptions{})
	if e != nil || posed.WorldCircle == nil {
		t.Fatal(e)
	}
	wc := posed.WorldCircle
	for i := 0; i < 3; i++ {
		if wc.Center[i].Cmp(world(c.Center)[i]) != 0 || wc.U[i].Cmp(m[i][0]) != 0 || wc.V[i].Cmp(m[i][1]) != 0 {
			t.Fatal("world circle")
		}
	}
	if wc.SquaredDistance.Cmp(c.SquaredDistance) != 0 || wc.Face != c.Face {
		t.Fatal("world owner/distance")
	}
	// An upper cap is closer although the full cone foot remains valid.
	cap, e := q.BoundConeAxisNearestCircleRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(11, 2)}, FilletedFrustumDistanceOptions{})
	if e != nil || cap.Circle != nil || !cap.Baseline.Exact || cap.Baseline.Upper.Cmp(r(1, 4)) != 0 {
		t.Fatal("adjacent closer cap", cap, e)
	}
	for _, point := range [][3]*big.Rat{{r(1, 100), r(0, 1), r(3, 1)}, {r(0, 1), r(0, 1), r(0, 1)}} {
		v, e := q.BoundConeAxisNearestCircleRational(ctx, point, FilletedFrustumDistanceOptions{})
		if e != nil || v.Circle != nil {
			t.Fatal("off-axis/unretained foot", e)
		}
	}
	c.Radius.SetInt64(999)
	fresh, e := q.BoundConeAxisNearestCircleRational(ctx, p, FilletedFrustumDistanceOptions{})
	if e != nil || fresh.Circle == nil || fresh.Circle.Radius.Cmp(r(81, 50)) != 0 {
		t.Fatal("owned circle", e)
	}
	bad, e := q.BoundConeAxisNearestCircleRational(ctx, p, FilletedFrustumDistanceOptions{MaxSupportChecks: 1})
	if e == nil || bad.Circle != nil || bad.Baseline.Lower != nil {
		t.Fatal("atomic budget")
	}
}
