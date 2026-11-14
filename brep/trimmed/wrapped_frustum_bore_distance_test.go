package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"plainsolid/geom"
)

func TestWrappedEccentricFrustumBoreBoundaryDistance(t *testing.T) {
	ctx := context.Background()
	limits := geom.FrustumDistanceOptions{SquaredTolerance: 1e-10}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 4, limits.Surface)
	if e != nil {
		t.Fatal(e)
	}
	outer, _, e = geom.DraftWrappedFrustum(ctx, outer, 0, 1, limits.Surface)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25}, 1, 4, limits.Surface)
	if e != nil {
		t.Fatal(e)
	}
	check := func(q WrappedFrustumBoreDistance, exact *big.Rat, near float64, owner int, err error) {
		t.Helper()
		if err != nil || q.Lower == nil || q.Upper == nil || q.Face != owner ||
			new(big.Rat).Sub(q.Upper, q.Lower).Cmp(ratInset(limits.SquaredTolerance)) > 0 {
			t.Fatal("uncertified retained boundary distance", q, err)
		}
		if exact != nil && (q.Lower.Cmp(exact) != 0 || q.Upper.Cmp(exact) != 0) {
			t.Fatal("exact retained boundary distance", q.Lower, q.Upper, exact)
		}
		lo, _ := q.Lower.Float64()
		hi, _ := q.Upper.Float64()
		if exact == nil && (lo > near+1e-14 || hi < near-1e-14) {
			t.Fatal("bound excludes independent radical distance", lo, hi, near)
		}
	}
	// A point one unit above an open center is sqrt(2) from the true
	// inner rim. Treating either removed cap disk as present would give 1.
	q, e := BoundPointWrappedEccentricFrustumBoreDistance(ctx, outer, inner, geom.Vec3{X: .25, Y: .25, Z: 5}, limits)
	check(q, big.NewRat(2, 1), 0, 3, e)
	if !q.Witness.RotationallyNonunique || q.Witness.Radius.Cmp(big.NewRat(1, 1)) != 0 || q.Witness.Height.Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal("axis witness must be an original rim", q.Witness)
	}
	// The offset projection is well inside the removed hole, but its
	// distance to the rim involves two irrational radial coordinates.
	q, e = BoundPointWrappedEccentricFrustumBoreDistance(ctx, outer, inner, geom.Vec3{Z: 5}, limits)
	check(q, nil, 1+math.Pow(1-math.Sqrt(.125), 2), 3, e)
	// A point whose XY lies in retained annular material projects straight
	// onto the cap at an original plane coordinate.
	q, e = BoundPointWrappedEccentricFrustumBoreDistance(ctx, outer, inner, geom.Vec3{X: 2, Z: 5}, limits)
	check(q, big.NewRat(1, 1), 0, 2, e)
	if q.Witness.CenterX.Cmp(big.NewRat(2, 1)) != 0 || q.Witness.Radius.Sign() != 0 {
		t.Fatal("retained annulus plane witness", q.Witness)
	}
	cavity, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25, Z: .5}, 1, 3, limits.Surface)
	if e != nil {
		t.Fatal(e)
	}
	body, ancestry, e := NewWrappedEccentricCircularFrustumBore(ctx, outer, cavity, 1e-8, limits.Surface)
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	q, e = body.BoundPointCanonicalWrappedFrustumBoreDistance(ctx, geom.Vec3{X: .25, Y: .25, Z: .75}, limits)
	check(q, big.NewRat(1, 16), 0, len(faces)-1-ancestry.InnerFaces[1], e)
	q, e = body.BoundPointCanonicalWrappedFrustumBoreDistance(ctx, geom.Vec3{X: .25, Y: .25, Z: 3.125}, limits)
	check(q, big.NewRat(9, 64), 0, len(faces)-1-ancestry.InnerFaces[2], e)
	short := limits
	short.MaxBisections = 1
	if q, e := BoundPointWrappedEccentricFrustumBoreDistance(ctx, outer, inner, geom.Vec3{Z: 5}, short); !errors.Is(e, geom.ErrConvergence) || q.Upper != nil {
		t.Fatal("bisection limit published a partial boundary witness", q, e)
	}
	short = limits
	short.Surface.MaxOperations = q.ExactOperations - 1
	if q, e := BoundPointWrappedEccentricFrustumBoreDistance(ctx, outer, cavity, geom.Vec3{X: .25, Y: .25, Z: 3.125}, short); !errors.Is(e, geom.ErrConvergence) || q.Upper != nil {
		t.Fatal("aggregate work limit published a partial boundary witness", q, e)
	}
}

func TestOCCTWrappedFrustumBoreBoundaryDistance(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_FRUSTUM_BORE_DISTANCE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_FRUSTUM_BORE_DISTANCE to the optional OCCT executable")
	}
	ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	raw, e := exec.CommandContext(ctx, program).CombinedOutput()
	if e != nil {
		t.Fatalf("independent OCCT bore distance: %v: %s", e, raw)
	}
	var result []float64
	if e := json.Unmarshal(raw, &result); e != nil || len(result) != 5 {
		t.Fatal("invalid OCCT distance record", string(raw), e)
	}
	for i, want := range [5]float64{2, 1 + math.Pow(1-math.Sqrt(.125), 2), 1, 1.0 / 16, 9.0 / 64} {
		if math.Abs(result[i]-want) > 1e-8 {
			t.Fatal("OCCT cut distance disagrees with the independent analytic fixture", i, result[i], want)
		}
	}
}

func TestNativeWrappedFrustumBoreRationalBoundaryDistance(t *testing.T) {
	ctx := context.Background()
	o := geom.FrustumDistanceOptions{SquaredTolerance: 1e-84, MaxBisections: 512}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 4, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25}, 1, 4, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	var saved bytes.Buffer
	if e = WriteJSON(&saved, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSONContext(ctx, &saved, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	faces := s.Faces()
	slices.Reverse(faces)
	s, e = NewSolidContext(ctx, s.Vertices(), s.Edges(), faces, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	proof, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	vertical2 := big.NewRat(1, 49)
	for _, sign := range []int64{-1, 1} {
		x := new(big.Rat).Add(big.NewRat(5, 4), new(big.Rat).Mul(big.NewRat(sign, 1), eps))
		p := [3]*big.Rat{x, big.NewRat(1, 4), big.NewRat(29, 7)}
		r, e := s.BoundPointCanonicalWrappedFrustumBoreDistanceRational(ctx, p, o)
		if e != nil || r.Lower == nil || r.Upper == nil {
			t.Fatal(r, e)
		}
		want := new(big.Rat).Set(vertical2)
		face := proof.OuterFaces[2]
		if sign < 0 {
			want.Add(want, new(big.Rat).Mul(eps, eps))
			face = proof.InnerFaces[0]
		}
		if r.Lower.Cmp(want) != 0 || r.Upper.Cmp(want) != 0 || r.Face != face {
			t.Fatal("sub-ULP inside/outside opening must retain exact distance and owner", sign, r, want, face)
		}
		if sign > 0 && (r.Witness.Radius.Sign() != 0 || r.Witness.CenterX.Cmp(x) != 0 || r.Witness.Height.Cmp(big.NewRat(4, 1)) != 0) {
			t.Fatal("rounded exact cap witness", r.Witness)
		}
		if sign < 0 && (r.Witness.Radius.Cmp(big.NewRat(1, 1)) != 0 || r.Witness.Height.Cmp(big.NewRat(4, 1)) != 0) {
			t.Fatal("the removed disk supplied a witness", r.Witness)
		}
		limited := o
		limited.Surface.MaxCoefficientBits = 64
		if no, e := s.BoundPointCanonicalWrappedFrustumBoreDistanceRational(ctx, p, limited); !errors.Is(e, geom.ErrConvergence) || no.Upper != nil {
			t.Fatal("precision budget must fail without a partial witness", no, e)
		}
	}
}
