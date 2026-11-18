//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRationalStationOriginalNormals(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	proof, bands, e := body.validateWrappedProfileStationLoft(ctx, geom.SurfacePatchOptions{}, false)
	if e != nil {
		t.Fatal(e)
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := range a {
			v.Add(v, new(big.Rat).Mul(a[i], b[i]))
		}
		return v
	}
	proportional := func(a, b [3]*big.Rat) bool {
		pivot := -1
		for i := range b {
			if b[i].Sign() != 0 {
				pivot = i
				break
			}
		}
		if pivot < 0 {
			return false
		}
		scale := new(big.Rat).Quo(a[pivot], b[pivot])
		if scale.Sign() <= 0 {
			return false
		}
		for i := range b {
			if a[i].Cmp(new(big.Rat).Mul(scale, b[i])) != 0 {
				return false
			}
		}
		return true
	}
	// Genuine noncircular diagonal: normalized(5/7,5/7), with tapered
	// outward direction proportional to(7,7,20) below and(7,7,-5) above.
	signs := [][2]int64{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}}
	for band := 0; band < 2; band++ {
		for j, sign := range signs {
			jet, e := bands[band].FundamentalPatch().EvaluateRational(ctx, r(int64(2*j+1), 2), r(1, 2), geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			got, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, jet.Point, WrappedRationalStationNormalOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if len(got.Normals) != 1 || got.Normals[0].Face != proof.History.WallFaces[band] {
				t.Fatal("original wall", got)
			}
			n := got.Normals[0]
			z := int64(20)
			if band == 1 {
				z = -5
			}
			want := [3]*big.Rat{r(7*sign[0], 1), r(7*sign[1], 1), r(z, 1)}
			if !proportional(n.Direction, want) || dot(n.Direction, jet.Du).Sign() != 0 || dot(n.Direction, jet.Dv).Sign() != 0 || dot(n.Direction, n.Direction).Cmp(n.SquaredLength) != 0 {
				t.Fatal("exact original tangent orthogonality", band, j, n)
			}
			epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
			for _, sgn := range []int64{-1, 1} {
				var p [3]*big.Rat
				for k := range p {
					delta := new(big.Rat).Mul(epsilon, n.Direction[k])
					delta.Mul(delta, r(sgn, 1))
					p[k] = new(big.Rat).Add(jet.Point[k], delta)
				}
				loc, e := body.LocateWrappedRationalStationLoftRational(ctx, p, WrappedRationalStationPointOptions{})
				want := parametric.Inside
				if sgn > 0 {
					want = parametric.Outside
				}
				if e != nil || loc.Location != want {
					t.Fatal("subfloat normal side", band, j, sgn, loc, e)
				}
			}
			saved := new(big.Rat).Set(got.Point[0])
			n.Direction[0].SetInt64(99)
			if got.Point[0].Cmp(saved) != 0 {
				t.Fatal("aliased input/normal")
			}
		}
	}
	point := func(x, z *big.Rat) [3]*big.Rat { return [3]*big.Rat{new(big.Rat).Add(r(1, 4), x), r(-1, 2), z} }
	for _, tc := range []struct {
		x, z   *big.Rat
		owners []int
	}{
		{r(0, 1), r(0, 1), []int{proof.History.CapFaces[0]}},
		{r(0, 1), r(3, 1), []int{proof.History.CapFaces[1]}},
		{r(1, 1), r(1, 1), []int{proof.History.WallFaces[0], proof.History.WallFaces[1]}},
		{r(3, 1), r(0, 1), []int{proof.History.CapFaces[0], proof.History.WallFaces[0]}},
		{r(2, 1), r(3, 1), []int{proof.History.CapFaces[1], proof.History.WallFaces[1]}},
	} {
		slices.Sort(tc.owners)
		q, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, point(tc.x, tc.z), WrappedRationalStationNormalOptions{})
		if e != nil || len(q.Normals) != len(tc.owners) {
			t.Fatal(q, e)
		}
		for i, n := range q.Normals {
			if n.Face != tc.owners[i] {
				t.Fatal("owner", n.Face, tc.owners)
			}
			var want [3]*big.Rat
			switch n.Face {
			case proof.History.CapFaces[0]:
				want = [3]*big.Rat{r(0, 1), r(0, 1), r(-1, 1)}
			case proof.History.CapFaces[1]:
				want = [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}
			case proof.History.WallFaces[0]:
				want = [3]*big.Rat{r(1, 1), r(0, 1), r(2, 1)}
			default:
				want = [3]*big.Rat{r(1, 1), r(0, 1), r(-1, 2)}
			}
			if !proportional(n.Direction, want) {
				t.Fatal("one-sided cardinal/crease/cap", n, want)
			}
		}
	}
	for _, p := range [][3]*big.Rat{point(r(0, 1), r(1, 1)), point(r(4, 1), r(1, 1))} {
		if _, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, p, WrappedRationalStationNormalOptions{}); e == nil {
			t.Fatal("nonboundary accepted")
		}
	}
}

func TestRationalStationNormalGuards(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{r(5, 4), r(-1, 2), r(1, 1)}
	for _, o := range []WrappedRationalStationNormalOptions{{MaxOperations: 1}, {MaxPointOperations: 1}, {Source: geom.SurfacePatchOptions{MaxOperations: 1}}} {
		q, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, p, o)
		if e == nil || len(q.Normals) != 0 || q.Point[0] != nil {
			t.Fatal("non-atomic budget", q, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := body.WrappedRationalStationBoundaryNormalsRational(cancelled, p, WrappedRationalStationNormalOptions{}); e == nil {
		t.Fatal("cancel")
	}
}

func TestRationalStationNormalOffDiagonal(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	// Neither the profile weight nor the rational evaluation parameters are
	// replaced by an ideal circular quadrant or rounded model coordinates.
	band := rationalStationBand(t, 0, 1, 3, 1, .6)
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{band}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	jet, e := band.FundamentalPatch().EvaluateRational(ctx, r(1, 3), r(1, 3), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	got, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, jet.Point, WrappedRationalStationNormalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Normals) != 1 {
		t.Fatal(got)
	}
	n := got.Normals[0].Direction
	for _, tangent := range [][3]*big.Rat{jet.Du, jet.Dv} {
		dot := new(big.Rat)
		for i := range n {
			dot.Add(dot, new(big.Rat).Mul(n[i], tangent[i]))
		}
		if dot.Sign() != 0 {
			t.Fatal("off-diagonal source tangent", dot)
		}
	}
	if n[0].Sign() <= 0 || n[1].Sign() <= 0 || n[2].Sign() <= 0 {
		t.Fatal("outward tapered normal", n)
	}
}

func TestRationalStationNormalSharedBudget(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	single, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, [3]*big.Rat{r(9, 4), r(-1, 2), r(1, 2)}, WrappedRationalStationNormalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	crease := [3]*big.Rat{r(5, 4), r(-1, 2), r(1, 1)}
	complete, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, crease, WrappedRationalStationNormalOptions{})
	if e != nil || complete.ExactOperations <= single.ExactOperations {
		t.Fatal("count", complete, e)
	}
	q, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, crease, WrappedRationalStationNormalOptions{MaxOperations: single.ExactOperations})
	if e == nil || len(q.Normals) != 0 {
		t.Fatal("budget reset at second original wall", q, e)
	}
}
