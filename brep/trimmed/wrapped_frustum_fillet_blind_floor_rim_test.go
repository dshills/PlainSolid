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

func TestFilletedFrustumBlindFloorRimDistance(t *testing.T) {
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
	opts := FilletedFrustumNestedDistanceOptions{Original: FilletedFrustumToroidDistanceOptions{Rims: FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}}

	for _, z := range []*big.Rat{r(9, 8), r(15, 8)} {
		p := [3]*big.Rat{r(5, 2), r(1, 2), z}
		got, e := q.ClosestBoundaryRational(ctx, p, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Kind != FilletedFrustumClosestRim || !got.NearestProven || !got.Converged {
			t.Fatalf("floor rim must be proved: kind%v nearest%v converged%v", got.Kind, got.NearestProven, got.Converged)
		}
		rim := got.Proof.Baseline.Baseline.Baseline.Rim
		// Independent D=(sqrt(1/2)-1/2)^2+(1/8)^2 =49/64-sqrt(1/2).
		A := r(49, 64)
		poly := func(d *big.Rat) *big.Rat { return fdSub(fdSquare(fdSub(A, d)), r(1, 2)) }
		if got.Upper.Cmp(A) >= 0 || poly(got.Lower).Sign() < 0 || poly(got.Upper).Sign() > 0 || rim.AzimuthSquared.Cmp(r(1, 2)) != 0 {
			t.Fatal("distance polynomial")
		}
		ci := 1
		if z.Cmp(r(3, 2)) > 0 {
			ci = 2
		}
		c := q.cuts[ci]
		if len(got.Faces) != 2 || len(rim.RimEdges) != 4 {
			t.Fatal("shared original circle ancestry")
		}
		seen := map[int]bool{}
		for _, f := range got.Faces {
			seen[f] = true
		}
		if !seen[c.wall] || !seen[c.floor] {
			t.Fatal("original floor/wall ownership")
		}
		state, e := q.ClassifyRational(ctx, p)
		if e != nil || state.Location != geom.BoreInside {
			t.Fatal("stock-side probe", state, e)
		}
		posed, e := placed.ClosestBoundaryRational(ctx, world(p), opts)
		if e != nil || posed.Kind != got.Kind || !posed.NearestProven || posed.Lower.Cmp(got.Lower) != 0 || posed.Upper.Cmp(got.Upper) != 0 {
			t.Fatal("world proof", e)
		}
		wr := posed.Proof.Fallback.Fallback.Fallback.WorldRim
		if wr == nil {
			t.Fatal("world shared rim")
		}
		for j := 0; j < 3; j++ {
			base, v := new(big.Rat), new(big.Rat)
			for i := 0; i < 3; i++ {
				base.Add(base, fdMul(m[i][j], fdSub(wr.Center[i], m[i][3])))
				v.Add(v, fdMul(m[i][j], wr.RadialNumerator[i]))
			}
			wantC := [3]*big.Rat{r(2, 1), r(0, 1), rim.Z}
			wantV := [3]*big.Rat{r(1, 4), r(1, 4), r(0, 1)}
			if base.Cmp(wantC[j]) != 0 || v.Cmp(wantV[j]) != 0 {
				t.Fatal("world rim algebra")
			}
		}
	}
	// Same radius but inside the finite wall height: rim is not closest.
	inside := [3]*big.Rat{r(5, 2), r(1, 2), r(7, 8)}
	got, e := q.ClosestBoundaryRational(ctx, inside, opts)
	if e != nil || got.Kind != FilletedFrustumClosestBore || !got.NearestProven {
		t.Fatal("inside axial wall", e)
	}
	// Beyond floor but within disk, the flat floor projection is closest.
	got, e = q.ClosestBoundaryRational(ctx, [3]*big.Rat{r(21, 10), r(1, 10), r(9, 8)}, opts)
	if e != nil || got.Kind != FilletedFrustumClosestRational || got.Upper.Cmp(r(1, 64)) != 0 {
		t.Fatal("inside floor disk", e)
	}
}
