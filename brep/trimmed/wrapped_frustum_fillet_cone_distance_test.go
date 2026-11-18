package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestFilletedFrustumIrrationalConeDistance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	r := big.NewRat
	for _, descending := range []bool{false, true} {
		R, delta, lower, upper := 8., 8., 15./16, 15./8
		if descending {
			R, delta, lower, upper = 16, -8, 15./8, 15./16
		}
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, R, 6, o)
		if e != nil {
			t.Fatal(e)
		}
		w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, delta, o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		cuts := []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}}
		if !descending {
			cuts = append(cuts, FilletedFrustumBoreCut{Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, FilletedFrustumBoreCut{Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true})
		}
		q, e := PrepareFilletedFrustumMaterial(ctx, s, lower, upper, cuts, o)
		if e != nil {
			t.Fatal(e)
		}
		opts := FilletedFrustumNestedDistanceOptions{Original: FilletedFrustumToroidDistanceOptions{Rims: FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}}
		z := r(3, 2)
		if descending {
			z = r(9, 2)
		}
		for _, xy := range [][2]int64{{10, 1}, {9, 4}} {
			p := [3]*big.Rat{r(xy[0], 1), r(xy[1], 1), z}
			S := r(xy[0]*xy[0]+xy[1]*xy[1], 1)
			got, e := q.BoundBoundaryConeDistanceRational(ctx, p, opts)
			if e != nil {
				t.Fatal(e)
			}
			if got.Cone == nil || !got.NearestProven || !got.Converged || got.Upper.Cmp(got.Baseline.Upper) >= 0 || got.Cone.Face != q.fillets.Lateral {
				t.Fatal(descending, got)
			}
			witness := got.Cone
			if witness.Radicand.Cmp(S) != 0 || fdSquare(witness.Root[0]).Cmp(S) > 0 || fdSquare(witness.Root[1]).Cmp(S) < 0 {
				t.Fatal(witness)
			}
			// Independent arithmetic in Q(sqrt(S)) verifies original support and
			// stationarity, rather than substituting rounded algebraic coordinates.
			type field [2]*big.Rat
			add := func(a, b field) field { return field{fdAdd(a[0], b[0]), fdAdd(a[1], b[1])} }
			sub := func(a, b field) field { return field{fdSub(a[0], b[0]), fdSub(a[1], b[1])} }
			mul := func(a, b field) field {
				return field{fdAdd(fdMul(a[0], b[0]), fdMul(S, fdMul(a[1], b[1]))), fdAdd(fdMul(a[0], b[1]), fdMul(a[1], b[0]))}
			}
			constant := func(a *big.Rat) field { return field{a, new(big.Rat)} }
			zero := func(a field) bool { return a[0].Sign() == 0 && a[1].Sign() == 0 }
			var point, difference [3]field
			for i := range point {
				point[i] = field{witness.Base[i], witness.RadicalVector[i]}
				difference[i] = sub(constant(p[i]), point[i])
			}
			radius := add(constant(r(int64(R), 1)), mul(constant(q.slope), point[2]))
			if !zero(sub(add(mul(point[0], point[0]), mul(point[1], point[1])), mul(radius, radius))) {
				t.Fatal("original cone equation")
			}
			tangent := [3]field{{new(big.Rat), new(big.Rat).Quo(fdMul(q.slope, p[0]), S)}, {new(big.Rat), new(big.Rat).Quo(fdMul(q.slope, p[1]), S)}, constant(r(1, 1))}
			stationary := constant(new(big.Rat))
			distance := constant(new(big.Rat))
			for i := range point {
				stationary = add(stationary, mul(difference[i], tangent[i]))
				distance = add(distance, mul(difference[i], difference[i]))
			}
			if !zero(stationary) || !zero(add(mul(difference[0], constant(new(big.Rat).Neg(p[1]))), mul(difference[1], constant(p[0])))) {
				t.Fatal("two tangent directions")
			}
			A := fdMul(fdAdd(S, r(100, 1)), r(9, 25))
			B := r(36, 5)
			if distance[0].Cmp(A) != 0 || distance[1].Cmp(new(big.Rat).Neg(B)) != 0 {
				t.Fatal("independent distance expression", distance)
			}
			poly := func(x *big.Rat) *big.Rat { return fdSub(fdSquare(fdSub(A, x)), fdMul(fdSquare(B), S)) }
			if got.Upper.Cmp(A) >= 0 || poly(got.Lower).Sign() < 0 || poly(got.Upper).Sign() > 0 {
				t.Fatal("algebraic distance enclosure")
			}
			lo, hi := witness.Root[0], witness.Root[1]
			if witness.RadicalVector[2].Sign() < 0 {
				lo, hi = hi, lo
			}
			zlo, zhi := fdAdd(witness.Base[2], fdMul(witness.RadicalVector[2], lo)), fdAdd(witness.Base[2], fdMul(witness.RadicalVector[2], hi))
			if zlo.Cmp(q.fillets.Circles[0].Tangency[1]) <= 0 || zhi.Cmp(q.fillets.Circles[1].Tangency[1]) >= 0 {
				t.Fatal("retained cone interval")
			}
			expected := geom.BoreOutside
			if xy[0] == 9 {
				expected = geom.BoreInside
			}
			material, e := q.ClassifyRational(ctx, p)
			if e != nil || material.Location != expected {
				t.Fatal(material, e)
			}
			witness.Base[0].SetInt64(100)
			again, e := q.BoundBoundaryConeDistanceRational(ctx, p, opts)
			if e != nil || again.Cone.Base[0].Cmp(r(100, 1)) == 0 {
				t.Fatal("owned coefficients", e)
			}
		}
		// A line projection beyond the actual cone/toroid seam is not retained.
		badZ := r(0, 1)
		if descending {
			badZ = r(6, 1)
		}
		rejected, e := q.BoundBoundaryConeDistanceRational(ctx, [3]*big.Rat{r(8, 1), r(1, 1), badZ}, opts)
		if e != nil || rejected.Cone != nil {
			t.Fatal("unbounded projection accepted", rejected, e)
		}
		rational, e := q.BoundBoundaryConeDistanceRational(ctx, [3]*big.Rat{r(10, 1), r(0, 1), z}, opts)
		if e != nil || rational.Cone != nil || !rational.NearestProven || rational.Lower.Sign() != 0 {
			t.Fatal("rational path changed", rational, e)
		}
		limited := opts
		limited.MaxNestedChecks = 1
		if v, e := q.BoundBoundaryConeDistanceRational(ctx, [3]*big.Rat{r(10, 1), r(1, 1), z}, limited); e == nil || v.Lower != nil || v.Cone != nil || v.Baseline.Lower != nil {
			t.Fatal("atomic budget", v, e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if v, e := q.BoundBoundaryConeDistanceRational(cancelled, [3]*big.Rat{r(10, 1), r(1, 1), z}, opts); e != context.Canceled || v.Lower != nil {
			t.Fatal(v, e)
		}
	}
}
