package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func nativeThreeCrossingsTangencyFixture(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// Subtract f to get -(u-1/8)^2(u-3/8)(u-5/8)(u-7/8).
	// The original roof product f*g has degree eight and fits the same
	// exact dyadic stock representation as earlier branch fixtures.
	g := []*big.Rat{
		big.NewRat(8297, 32768), big.NewRat(1255, 4096), big.NewRat(35, 256),
		big.NewRat(-47, 32), big.NewRat(17, 8), big.NewRat(-1, 1),
	}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func TestSourceGraphQuinticTrimsPreserveExactRestrictions(t *testing.T) {
	ctx := context.Background()
	for axis := 0; axis < 2; axis++ {
		base, a, b := nativeThreeCrossingsTangencyFixture(t, axis)
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			patches, e := prepareSharedBaseGraphContact(ctx, base, roofs[0], roofs[1], intersect.SplinePlaneOptions{})
			if e != nil {
				t.Fatal("exact identity chart and common positive base", axis, order, e)
			}
			proof, e := geom.DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
			if e != nil || !proof.Proven || proof.ContactOrder != [4]int{2, 1, 1, 1} {
				t.Fatal("solid stocks retain the complete quintic contact arrangement", axis, order, e)
			}
			uv, _, e := proof.RepresentedVertices()
			if e != nil {
				t.Fatal("all required terminal and contact coordinates are lossless", axis, order, e)
			}
			for i, source := range proof.Edges {
				var controls [][3]*big.Rat
				for _, p := range source.Controls {
					controls = append(controls, [3]*big.Rat{p[0], p[1], big.NewRat(1, 1)})
				}
				// The ordinary constructor keeps its original degree limit.
				// Exact loop validation can inspect all six controls.
				if _, e := trimvalidate.ExactBezier(controls); e != nil {
					t.Fatal("the exact validation descriptor accepts the original restriction", i, e)
				}
				curve, e := NewExactBezierUV(controls)
				if source.Branch == 0 {
					if e != nil || len(controls) != 4 || curve.ends != ([2]geom.Vec2{uv[source.Start], uv[source.End]}) {
						t.Fatal("ordinary cubic restrictions still fit the native trim", i, e)
					}
					continue
				}
				if len(controls) != 6 || !errors.Is(e, geom.ErrInvalid) || len(curve.HomogeneousControls()) != 0 {
					t.Fatal("the exact quintic restriction must fail ordinary native trim construction", i, e)
				}
				if source.Start >= 4 && source.End >= 4 {
					checkQuinticSplitPoint(t, patches, proof, source)
				}
			}
			pieces, e := polynomialNodePieces(proof.Edges[:], uv[:])
			if e != nil {
				t.Fatal("source-proved quintic trim creation", axis, order, e)
			}
			checkQuinticGraphPieces(t, proof, uv, pieces)
			checkQuinticGraphOrdinaryCodec(t, pieces[0][0].c.(ExactBezierUV), pieces[5][0].c.(ExactBezierUV))
		}
	}
}

func checkQuinticSplitPoint(t *testing.T, patches [2]*geom.SurfacePatch, proof geom.SplineGraphPolynomialThreeCrossingsTangency, source geom.SplineGraphPolynomialPiece) {
	t.Helper()
	axis := proof.FreeAxis
	first, last := proof.Vertices[source.Start].Parameters[axis], proof.Vertices[source.End].Parameters[axis]
	free := new(big.Rat).Quo(new(big.Rat).Add(first, last), big.NewRat(2, 1))
	dependent := fourPolynomialCutBranchValue(proof.Power[source.Branch], free)
	var uv [2]*big.Rat
	uv[axis], uv[1-axis] = free, dependent
	ctx := context.Background()
	a, e := patches[0].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := patches[1].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
	if e != nil || !reflect.DeepEqual(a.Point, b.Point) {
		t.Fatal("both original supports lift the midpoint identically", e)
	}
	if _, e := geom.RepresentedExactPoint(a.Point); e != nil {
		t.Fatal("the eventual split also has lossless model coordinates", e)
	}
}

func checkQuinticGraphPieces(t *testing.T, proof geom.SplineGraphPolynomialThreeCrossingsTangency, uv [8]geom.Vec2, pieces [][]openSectionPiece) {
	t.Helper()
	for i, source := range proof.Edges {
		chain := pieces[i]
		wantParts := 1
		if source.Start >= 4 && source.End >= 4 {
			wantParts = 2
		}
		if len(chain) != wantParts || chain[0].a != uv[source.Start] || chain[len(chain)-1].b != uv[source.End] ||
			(len(chain) == 2 && chain[0].b != chain[1].a) {
			t.Fatal("complete native restriction with distinct source endpoints", i, chain)
		}
		first := proof.Vertices[source.Start].Parameters[proof.FreeAxis]
		last := proof.Vertices[source.End].Parameters[proof.FreeAxis]
		width := new(big.Rat).Quo(new(big.Rat).Sub(last, first), big.NewRat(int64(wantParts), 1))
		power := proof.Power[source.Branch]
		differentiated := make([]*big.Rat, len(power)-1)
		for k, c := range power[1:] {
			differentiated[k] = new(big.Rat).Mul(c, big.NewRat(int64(k+1), 1))
		}
		for j, piece := range chain {
			curve, ok := piece.c.(ExactBezierUV)
			if !ok || len(curve.control) != len(source.Controls) {
				t.Fatal("exact subdivision preserves the original degree", i, j)
			}
			for _, p := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 5), big.NewRat(3, 4), big.NewRat(1, 1)} {
				v, d, e := curve.EvaluateRational(p)
				free := new(big.Rat).Add(first, new(big.Rat).Mul(width, new(big.Rat).Add(big.NewRat(int64(j), 1), p)))
				dependent := fourPolynomialCutBranchValue(power, free)
				slope := new(big.Rat).Mul(width, fourPolynomialCutBranchValue(differentiated, free))
				if e != nil || v[proof.FreeAxis].Cmp(free) != 0 || v[1-proof.FreeAxis].Cmp(dependent) != 0 ||
					d[proof.FreeAxis].Cmp(width) != 0 || d[1-proof.FreeAxis].Cmp(slope) != 0 {
					t.Fatal("exact parameter mapping and chain rule on the original branch", i, j, p, e)
				}
			}
		}
	}
}

func checkQuinticGraphOrdinaryCodec(t *testing.T, cubic, quintic ExactBezierUV) {
	t.Helper()
	if a, b, e := quintic.Split(big.NewRat(1, 5)); !errors.Is(e, geom.ErrUnrepresentable) ||
		len(a.HomogeneousControls()) != 0 || len(b.HomogeneousControls()) != 0 {
		t.Fatal("exact quintic splits still require lossless native endpoints", e)
	}
	oversized := append(quintic.HomogeneousControls(), quintic.control[len(quintic.control)-1])
	if _, e := newExactBezierUV(oversized, maxSourceGraphBezierControls); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("source graph trims still have a fixed six-control limit", e)
	}
	encode := func(curve ExactBezierUV) ([]nativeUse, error) {
		return (&trimEncoder{}).encodeUses([]Use{{UVStart: curve.ends[0], UVEnd: curve.ends[1], Curve: curve}})
	}
	decoder := trimDecoder{ctx: context.Background(), version: 6, maxControls: 6}
	original, e := encode(cubic)
	if e != nil {
		t.Fatal(e)
	}
	uses, e := decoder.decodeUses(original, 0)
	if e != nil {
		t.Fatal(e)
	}
	reencoded, e := encode(uses[0].Curve.(ExactBezierUV))
	if e != nil {
		t.Fatal(e)
	}
	want, e := json.Marshal(original)
	if e != nil {
		t.Fatal(e)
	}
	got, e := json.Marshal(reencoded)
	if e != nil || !bytes.Equal(got, want) {
		t.Fatal("ordinary cubic record is byte-canonical", e)
	}

	if _, e := encode(quintic); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("ordinary native encoder cannot admit source-only quintic trims", e)
	}
	raw := nativeUse{UVStart: quintic.ends[0], UVEnd: quintic.ends[1], ExactBezier: make([][3]string, len(quintic.control))}
	for i, p := range quintic.control {
		for j, c := range p {
			raw.ExactBezier[i][j] = c.RatString()
		}
	}
	if got, e := decoder.decodeUses([]nativeUse{raw}, 0); got != nil || e == nil {
		t.Fatal("ordinary native decoder cannot admit a raw quintic record", e)
	}
}
