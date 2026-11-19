//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestMultiSpanMappedRepair(t *testing.T) {
	slowtest.Skip(t)
	for _, rev := range []bool{false, true} {
		for _, local := range []*big.Rat{big.NewRat(1, 8), big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(5, 8), big.NewRat(13, 16)} {
			t.Run(local.String()+map[bool]string{false: "forward", true: "reverse"}[rev], func(t *testing.T) {
				ctx := context.Background()
				s, edge, owner, original := multispanSharedSeamFixture(t, 3, rev)
				cut := new(big.Rat).Set(local)
				for _, u := range s.faces[owner].Outer {
					if u.Edge == edge && u.Reversed {
						cut.Sub(big.NewRat(1, 1), local)
					}
				}
				split, h, e := s.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{})
				if e != nil {
					t.Fatal("split", e)
				}
				reload := func(x Solid) Solid {
					var b, c bytes.Buffer
					if e := WriteJSON(&b, x); e != nil {
						t.Fatal(e)
					}
					y, e := ReadJSON(bytes.NewReader(b.Bytes()), JSONLimits{})
					if e != nil {
						t.Fatal(e)
					}
					if e = WriteJSON(&c, y); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
						t.Fatal("reload", e)
					}
					return y
				}
				split = reload(split)
				merged, _, e := split.MergeExactBezierTrimEdges(ctx, h.First, h.Second, cut)
				if e != nil {
					t.Fatal("merge", e)
				}
				merged = reload(merged)
				found := false
				for _, u := range merged.faces[owner].Outer {
					if q, ok := u.Curve.(MultiSpanMappedSplineUV); ok {
						found = true
						if !sameAuthoredBezierSpline(q.source.Curve, original.source.Curve) || q.source.Range != original.source.Range || q.source.Reversed != rev {
							t.Fatal("source changed")
						}
						if e := proveMappedRepairCurve(ctx, q.curve, original.curve, geom.SurfacePatchOptions{}); e != nil {
							t.Fatal(e)
						}
					}
				}
				if !found {
					t.Fatal("lost original")
				}
				for _, x := range []Solid{split, merged} {
					o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(x.faces))}
					for fi := range o.Faces {
						o.Faces[fi].Face = MixedExactMultiSpanMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 4}, Face: MixedExactCurvedMeshOptions{BoundaryModelBudget: .01, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}
					}
					for fi, f := range x.faces {
						for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
							cuts := make([][]*big.Rat, len(loop))
							for i := range cuts {
								for k := 0; k <= 4; k++ {
									cuts[i] = append(cuts[i], big.NewRat(int64(k), 4))
								}
							}
							o.Faces[fi].Face.Face.Cuts = append(o.Faces[fi].Face.Face.Cuts, cuts)
						}
					}
					r, e := x.TessellateMixedClosedCertified(ctx, o)
					if e != nil {
						t.Fatal("mesh", e)
					}
					patch, e := exactSplineSupportPatch(x.faces[owner].Surface, geom.SurfacePatchOptions{})
					if e != nil {
						t.Fatal(e)
					}
					samples := 0
					for _, use := range r.Network.Faces[owner].Coedges {
						_, raw, ok := mappedRepairSource(x.faces[owner].Outer[use.Use].Curve)
						if !ok {
							continue
						}
						if !sameAuthoredBezierSpline(raw.Curve, original.source.Curve) || raw.Reversed != rev {
							t.Fatal("child source changed")
						}
						for j, p := range use.AuthoredParameters {
							uv := authoredMixedSplinePoint(t, raw, p)
							point, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
							if e != nil {
								t.Fatal(e)
							}
							id := r.Network.FaceVertices[owner][use.Vertices[j]]
							for k := 0; k < 3; k++ {
								if point.Point[k].Cmp(r.Network.ExactPoints[id][k]) != 0 {
									t.Fatal("original source sample mismatch")
								}
							}
							samples++
						}
					}
					if samples < 3 {
						t.Fatal("missing authored samples")
					}
					if !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 {
						t.Fatal("contact")
					}
				}
			})
		}
	}
}

func TestMultiSpanMappedRepairGuards(t *testing.T) {
	ctx := context.Background()
	s, edge, owner, q := multispanSharedSeamFixture(t, 3, false)
	cut := big.NewRat(1, 16)
	for _, u := range s.faces[owner].Outer {
		if u.Edge == edge && u.Reversed {
			cut.Sub(big.NewRat(1, 1), cut)
		}
	}
	if out, _, e := s.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{}); e == nil || len(out.faces) != 0 {
		t.Fatal("nonrepresentable source endpoint accepted")
	}
	if out, _, e := s.SplitExactBezierTrimEdge(ctx, edge, big.NewRat(1, 4), edgecheck.Options{MaxWork: 1}); e == nil || len(out.faces) != 0 {
		t.Fatal("quota not atomic")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := restrictMultiSpanMappedSource(canceled, q, new(big.Rat), big.NewRat(1, 1), geom.SurfacePatchOptions{}); e == nil || out != nil {
		t.Fatal("cancellation not atomic")
	}
	a, e := restrictMultiSpanMappedSource(ctx, q, new(big.Rat), big.NewRat(1, 4), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := restrictMultiSpanMappedSource(ctx, q, big.NewRat(1, 4), big.NewRat(1, 1), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if out, _, e := joinPiecewiseMappedSources(ctx, a, b, big.NewRat(1, 3)); e == nil || out != nil {
		t.Fatal("forged parent station accepted")
	}
	if out, _, e := joinPiecewiseMappedSources(ctx, b, a, big.NewRat(3, 4)); e == nil || out != nil {
		t.Fatal("reordered ranges accepted")
	}
}

func TestMultiSpanMappedMaximumRepair(t *testing.T) {
	ctx := context.Background()
	_, _, _, q := multispanSharedSeamFixture(t, 8, true)
	cut := big.NewRat(3, 8)
	a, e := restrictMultiSpanMappedSource(ctx, q, new(big.Rat), cut, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := restrictMultiSpanMappedSource(ctx, q, cut, big.NewRat(1, 1), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, _, e := joinPiecewiseMappedSources(ctx, a, b, cut)
	if e != nil {
		t.Fatal(e)
	}
	restored, ok := c.(MultiSpanMappedSplineUV)
	if !ok || len(restored.parts) != 8 || !sameAuthoredBezierSpline(restored.source.Curve, q.source.Curve) {
		t.Fatal("maximum source not reconstructed")
	}
	for i, p := range restored.parts {
		if p.weight.Cmp(q.parts[i].weight) != 0 {
			t.Fatal("speed changed")
		}
	}
	if e = proveMappedRepairCurve(ctx, restored.curve, q.curve, geom.SurfacePatchOptions{}); e != nil {
		t.Fatal(e)
	}
}
