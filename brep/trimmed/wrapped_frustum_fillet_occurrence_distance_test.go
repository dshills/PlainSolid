//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestFilletedFrustumOccurrenceDistance(t *testing.T) {
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
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	placed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		return [3]*big.Rat{fdAdd(fdAdd(fdMul(r(3, 5), p[0]), fdMul(r(4, 5), p[2])), r(1, 3)), fdSub(p[1], r(2, 7)), fdAdd(fdAdd(fdMul(r(-4, 5), p[0]), fdMul(r(3, 5), p[2])), r(5, 11))}
	}
	floor := world([3]*big.Rat{r(2, 1), r(0, 1), r(9, 8)})
	rational, e := placed.BoundBoundaryDistanceRational(ctx, floor, FilletedFrustumDistanceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !rational.Exact || rational.Lower.Cmp(r(1, 64)) != 0 || rational.Upper.Cmp(r(1, 64)) != 0 || !reflect.DeepEqual(rational.WitnessFaces, []int{q.cuts[1].floor}) {
		t.Fatal(rational)
	}
	expected := [3]*big.Rat{r(7, 3), r(-2, 7), r(-6, 11)}
	for i := range expected {
		if rational.WorldWitness[i].Cmp(expected[i]) != 0 {
			t.Fatal(rational.WorldWitness, expected)
		}
	}
	membership, e := placed.ClassifyRational(ctx, rational.WorldWitness)
	if e != nil || membership.Location != geom.BoreBoundary || !reflect.DeepEqual(membership.Faces, rational.WitnessFaces) {
		t.Fatal(membership, e)
	}
	// The cone/fillet seam keeps both original owners after oblique placement.
	c := q.fillets.Circles[0]
	seam := world([3]*big.Rat{fdAdd(c.Tangency[0], fdMul(r(1, 64), c.Normal[0])), r(0, 1), fdAdd(c.Tangency[1], fdMul(r(1, 64), c.Normal[1]))})
	seamDistance, e := placed.BoundBoundaryDistanceRational(ctx, seam, FilletedFrustumDistanceOptions{})
	if e != nil || !seamDistance.Exact || seamDistance.Lower.Cmp(r(1, 4096)) != 0 || !reflect.DeepEqual(seamDistance.WitnessFaces, []int{q.fillets.Lateral, q.fillets.Blends[0]}) {
		t.Fatal(seamDistance, e)
	}
	opts := FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}
	p := world([3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)})
	got, e := placed.BoundBoundaryRimDistanceRational(ctx, p, opts)
	if e != nil {
		t.Fatal(e)
	}
	if got.WorldRim == nil || got.WorldRationalWitness[0] != nil || !got.NearestProven || !got.Converged || got.Lower.Cmp(got.Original.Lower) != 0 || got.Upper.Cmp(got.Original.Upper) != 0 {
		t.Fatal(got)
	}
	wr := got.WorldRim
	for i, v := range [3]*big.Rat{r(-13, 15), r(-2, 7), r(113, 55)} {
		if wr.Center[i].Cmp(v) != 0 {
			t.Fatal(wr.Center)
		}
	}
	for i, v := range [3]*big.Rat{r(3, 100), r(1, 20), r(-1, 25)} {
		if wr.RadialNumerator[i].Cmp(v) != 0 {
			t.Fatal(wr.RadialNumerator)
		}
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := range a {
			v.Add(v, fdMul(a[i], b[i]))
		}
		return v
	}
	if dot(wr.Normal, wr.Normal).Cmp(r(1, 1)) != 0 || dot(wr.Normal, wr.RadialNumerator).Sign() != 0 || dot(wr.RadialNumerator, wr.RadialNumerator).Cmp(fdMul(fdSquare(wr.Radius), wr.AzimuthSquared)) != 0 {
		t.Fatal("world circle identities", wr)
	}
	// Independent world-space distance expression is A+B/sqrt(q).
	// Check it equals457/1600-sqrt(1/50) without floating-point coordinates.
	var delta [3]*big.Rat
	for i := range p {
		delta[i] = fdSub(p[i], wr.Center[i])
	}
	a := fdAdd(dot(delta, delta), new(big.Rat).Quo(dot(wr.RadialNumerator, wr.RadialNumerator), wr.AzimuthSquared))
	b := fdMul(r(-2, 1), dot(delta, wr.RadialNumerator))
	if a.Cmp(r(457, 1600)) != 0 || b.Cmp(r(-1, 50)) != 0 || wr.AzimuthSquared.Cmp(r(1, 50)) != 0 {
		t.Fatal(a, b, wr.AzimuthSquared)
	}
	if !reflect.DeepEqual(got.WitnessFaces, []int{q.fillets.Caps[0], q.cuts[0].wall}) || !reflect.DeepEqual(wr.RimEdges, got.Original.Rim.RimEdges) {
		t.Fatal("owners", got)
	}
	// Exact rational branch of the extended API stays explicitly rational.
	v, e := placed.BoundBoundaryRimDistanceRational(ctx, floor, opts)
	if e != nil || v.WorldRim != nil || v.WorldRationalWitness[0] == nil || !v.NearestProven {
		t.Fatal(v, e)
	}
	// A source interval-only result is never promoted by rigid placement.
	ambiguous := world([3]*big.Rat{r(1, 1), r(1, 1), r(3, 1)})
	v, e = placed.BoundBoundaryRimDistanceRational(ctx, ambiguous, opts)
	if e != nil {
		t.Fatal(e)
	}
	if v.NearestProven != v.Original.NearestProven || v.NearestProven {
		t.Fatal(v)
	}
	wr.Center[0].SetInt64(100)
	wr.RadialNumerator[0].SetInt64(100)
	again, e := placed.BoundBoundaryRimDistanceRational(ctx, p, opts)
	if e != nil || again.WorldRim.Center[0].Cmp(r(-13, 15)) != 0 {
		t.Fatal(again, e)
	}
	bad := opts
	bad.MaxRimChecks = 1
	if v, e = placed.BoundBoundaryRimDistanceRational(ctx, p, bad); e == nil || v.Lower != nil || v.WorldRim != nil || v.Original.Lower != nil {
		t.Fatal("atomic budget", v, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if v, e = placed.BoundBoundaryRimDistanceRational(cancelled, p, opts); e != context.Canceled || v.Lower != nil {
		t.Fatal(v, e)
	}
	p[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 4097))
	if v, e = placed.BoundBoundaryRimDistanceRational(ctx, p, opts); e == nil || v.WorldRim != nil {
		t.Fatal("transform limit", v, e)
	}
}
