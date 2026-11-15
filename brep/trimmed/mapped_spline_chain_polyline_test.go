package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"testing"
)

func TestMappedSplineChainPolyline(t *testing.T) {
	ctx := context.Background()
	a, ca := mappedChainFixture(t, false)
	b, cb := mappedChainFixture(t, true)
	proof, e := CertifyMappedSplineChainAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var previous int
	for _, tol := range []float64{.02, .005} {
		r, e := proof.TessellateCertified(ctx, MappedChainPolylineOptions{Deflection: tol})
		if e != nil {
			t.Fatal(e)
		}
		if r.MaximumError > tol || len(r.KnotVertices) != 4 || len(r.Segments) != len(r.Vertices)-1 {
			t.Fatal("coverage")
		}
		if previous > len(r.Segments) {
			t.Fatal("refinement")
		}
		previous = len(r.Segments)
		for i, id := range r.KnotVertices {
			if r.Vertices[id].ModelParameter.Cmp(proof.cuts[i]) != 0 {
				t.Fatal("lost original station")
			}
		}
		for i, v := range r.Vertices {
			if i > 0 && v.ModelParameter.Cmp(r.Vertices[i-1].ModelParameter) <= 0 {
				t.Fatal("parameter order")
			}
			for owner, c := range []MappedSplineChain{ca, cb} {
				want, e := c.AuthoredParameter(v.ModelParameter)
				if e != nil || want.Cmp(v.AuthoredParameters[owner]) != 0 {
					t.Fatal("raw provenance", e)
				}
			}
		}
		for i, s := range r.Segments {
			if s.Vertices != [2]int{i, i + 1} {
				t.Fatal("shared incidence")
			}
			left, right := r.Vertices[i], r.Vertices[i+1]
			if left.ModelParameter.Cmp(proof.cuts[s.Cell]) < 0 || right.ModelParameter.Cmp(proof.cuts[s.Cell+1]) > 0 {
				t.Fatal("crossed original cell")
			}
			for _, f := range []*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(3, 4)} {
				tau := new(big.Rat).Add(left.ModelParameter, new(big.Rat).Mul(f, new(big.Rat).Sub(right.ModelParameter, left.ModelParameter)))
				var world [2][3]*big.Rat
				for owner, c := range []MappedSplineChain{ca, cb} {
					raw, e := c.AuthoredParameter(tau)
					if e != nil {
						t.Fatal(e)
					}
					uv := authoredMixedSplinePoint(t, c.parts[0].source, raw)
					x, y := uv[0], uv[1]
					if owner == 1 {
						x, y = y, x
					}
					world[owner] = [3]*big.Rat{x, y, new(big.Rat).Mul(x, y)}
				}
				sum := new(big.Rat)
				l := []float64{left.Point.X, left.Point.Y, left.Point.Z}
				h := []float64{right.Point.X, right.Point.Y, right.Point.Z}
				for k := 0; k < 3; k++ {
					if world[0][k].Cmp(world[1][k]) != 0 {
						t.Fatal("owners disagree")
					}
					chord := new(big.Rat).Add(ratInset(l[k]), new(big.Rat).Mul(f, new(big.Rat).Sub(ratInset(h[k]), ratInset(l[k]))))
					d := new(big.Rat).Sub(world[0][k], chord)
					sum.Add(sum, new(big.Rat).Mul(d, d))
				}
				bound := ratInset(s.ErrorUpper)
				if sum.Cmp(new(big.Rat).Mul(bound, bound)) > 0 {
					t.Fatal("independent exact chord witness exceeds bound")
				}
			}
		}
		t.Logf("tol=%g segments=%d bound=%g counted operations=%d", tol, len(r.Segments), r.MaximumError, r.ExactOperations)
	}
	for _, o := range []MappedChainPolylineOptions{{MaxSegments: 1}, {MaxSegments: 8, Deflection: .005}, {MaxOperations: 1}, {MaxOperations: 8000, Deflection: .005}, {Deflection: -1}, {MaxDepth: -1}} {
		out, e := proof.TessellateCertified(ctx, o)
		if e == nil || len(out.Vertices) != 0 || len(out.Segments) != 0 {
			t.Fatal("atomic limits")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := proof.TessellateCertified(canceled, MappedChainPolylineOptions{}); e == nil || len(out.Vertices) != 0 {
		t.Fatal("cancel")
	}
	if out, e := (MappedSplineChainAgreement{}).TessellateCertified(ctx, MappedChainPolylineOptions{}); e == nil || len(out.Vertices) != 0 {
		t.Fatal("unproved input")
	}
}
