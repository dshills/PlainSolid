//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type SolidExactQuadraticMeshOptions struct {
	Shell          ExactQuadraticShellMeshOptions
	ModelAgreement edgecheck.Options // Exact per-use model-edge proof limits.
}

// CertifiedSolidExactQuadraticMesh maps certified shell seams/corners back to
// the Solid's original edge/vertex indices. OriginalTrim maps normalized chart
// trim indices to original face use indices, flattened outer then holes. ReparameterizedFaces marks
// reflected U charts used to preserve physically reversed face orientation.
// SourceParameters maps each normalized output trim to its original p-curve
// parameter (SplineUV ranges included). No topology tolerance weld is used.
type CertifiedSolidExactQuadraticMesh struct {
	CertifiedEmbeddedExactQuadraticShell
	OriginalEdges, OriginalVertices []int
	ReparameterizedFaces            []bool
	SourceParameters                [][]ExactTrimParameterMap
	ModelEdgeUsesCertified          int
	RationalCorners                 []RationalVertex // Independent exact model identities; display points are rounded.
}

// TessellateExactQuadraticShellCertified adapts a stored spline Solid to the
// native closed-shell certificate. Supported faces have one outer loop, optional holes,
// and straight, SplineUV or ExactQuadraticUV trims. Closed/collapsed edges
// and same-face seams reject. Stored UV/model corner coordinates must agree
// exactly with rational support evaluation. Native rational endpoint evidence
// from RationalVertices overrides only the rounded display coordinate; distinct
// topology vertices sharing a display coordinate reject. Every accepted rounded
// corner is included in incident triangle correspondence bounds. Every model-edge use
// must also pass an exact continuous agreement proof before mesh assembly.
// Surface placements and reversed face charts preserve exact source maps.
// The returned certificate proves display embedding and source-image distance,
// not generic source-shell embedding or exact source mass.
func (s Solid) TessellateExactQuadraticShellCertified(ctx context.Context, o SolidExactQuadraticMeshOptions) (CertifiedSolidExactQuadraticMesh, error) {
	empty := CertifiedSolidExactQuadraticMesh{}
	if ctx == nil || len(s.faces) < 2 || len(s.faces) > 16 || len(s.edges) < 1 || len(s.edges) > 32 || len(s.vertices) > 64 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	limits := o.Shell.Join.Face.Composition.SurfacePatchOptions
	faces := make([]ExactQuadraticShellFace, len(s.faces))
	originalTrims := make([][]int, len(s.faces))
	parameterMaps := make([][]ExactTrimParameterMap, len(s.faces))
	reflected := make([]bool, len(s.faces))
	type ownership struct{ face, trim, start, end int }
	uses := make([][]ownership, len(s.edges))
	usedVertices := make([]bool, len(s.vertices))
	exactCorners := make([][3]*big.Rat, len(s.vertices))
	displayOwner := map[geom.Vec3]int{}
	for i, p := range s.vertices {
		if !p.IsFinite() {
			return empty, geom.ErrInvalid
		}
		if _, exists := displayOwner[p]; exists {
			return empty, geom.ErrDegenerate
		}
		displayOwner[p] = i
		exactCorners[i] = exactBoundPoint(p)
	}
	rationalCorners, err := s.RationalVertices(ctx)
	if err != nil {
		return empty, err
	}
	for _, v := range rationalCorners {
		exactCorners[v.Vertex] = v.Point
	}
	proofs := 0
	for fi, f := range s.faces {
		if len(f.Outer) < 2 {
			return empty, geom.ErrConvergence
		}
		base, placement := parametric.SurfaceMapping(f.Surface)
		surface, ok := base.(*geom.BSplineSurface)
		if !ok || surface == nil {
			return empty, geom.ErrConvergence
		}
		patch, e := geom.NewSurfacePatch(surface, limits)
		if e != nil {
			return empty, e
		}
		if placement != geom.Identity() {
			patch, e = patch.Transformed(placement)
			if e != nil {
				return empty, e
			}
		}
		domain := patch.Domain()
		if f.Reversed {
			patch, e = patch.Reparameterize(domain, true, false, limits)
			if e != nil {
				return empty, e
			}
			reflected[fi] = true
		}
		faces[fi].Patch = patch
		loops := append([][]Use{f.Outer}, f.Holes...)
		baseIndex := 0
		for li, loop := range loops {
			var nativeLoop []ExactQuadraticUV
			for j := range loop {
				index := j
				if f.Reversed {
					index = len(loop) - 1 - j
				}
				u := loop[index]
				if !uvFinite(u.UVStart) || !uvFinite(u.UVEnd) {
					return empty, geom.ErrInvalid
				}
				if u.Edge < 0 || u.Edge >= len(s.edges) {
					return empty, geom.ErrInvalid
				}
				edge := s.edges[u.Edge]
				if edge.Start < 0 || edge.End < 0 || edge.Start >= len(s.vertices) || edge.End >= len(s.vertices) || edge.Start == edge.End || edge.Curve == nil {
					return empty, geom.ErrConvergence
				}
				var h [3][3]*big.Rat
				rationalEndpoints := false
				mapping := ExactTrimParameterMap{Offset: new(big.Rat), Scale: big.NewRat(1, 1), Degree: 2}
				// Normalize pointer forms for the exact model-edge proof as well.
				switch curve := u.Curve.(type) {
				case nil:
					h = [3][3]*big.Rat{{ratInset(u.UVStart.X), ratInset(u.UVStart.Y), big.NewRat(1, 1)}, {nil, nil, big.NewRat(1, 1)}, {ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y), big.NewRat(1, 1)}}
					for k := 0; k < 2; k++ {
						h[1][k] = new(big.Rat).Quo(new(big.Rat).Add(h[0][k], h[2][k]), big.NewRat(2, 1))
					}
					mapping.Degree = 1
				case ExactQuadraticUV:
					h = curve.HomogeneousControls()
					rationalEndpoints = curve.rationalEndpoints
				case *ExactQuadraticUV:
					if curve == nil {
						return empty, geom.ErrInvalid
					}
					h = curve.HomogeneousControls()
					rationalEndpoints = curve.rationalEndpoints
					u.Curve = *curve
				case SplineUV:
					h, e = quadraticTrimSourceControls(curve)
					if e != nil {
						return empty, e
					}
					mapping = solidSplineParameterMap(curve)
				case *SplineUV:
					if curve == nil {
						return empty, geom.ErrInvalid
					}
					h, e = quadraticTrimSourceControls(*curve)
					if e != nil {
						return empty, e
					}
					u.Curve = *curve
					mapping = solidSplineParameterMap(*curve)
				default:
					return empty, geom.ErrConvergence
				}
				if h[0][0] == nil {
					return empty, geom.ErrInvalid
				}
				for side, point := range []geom.Vec2{u.UVStart, u.UVEnd} {
					for axis, value := range []float64{point.X, point.Y} {
						q := new(big.Rat).Quo(h[2*side][axis], h[2*side][2])
						display, _ := q.Float64()
						if (!rationalEndpoints && q.Cmp(ratInset(value)) != 0) || (rationalEndpoints && display != value) {
							return empty, geom.ErrConvergence
						}
					}
				}
				proven, e := compareSplineEdgeUse(ctx, edge, f.Surface, u, o.ModelAgreement)
				if e != nil {
					return empty, e
				}
				if !proven {
					return empty, geom.ErrConvergence
				}
				proofs++
				if f.Reversed {
					sum := new(big.Rat).Add(ratInset(domain.UMin), ratInset(domain.UMax))
					for k := range h {
						h[k][0] = new(big.Rat).Sub(new(big.Rat).Mul(sum, h[k][2]), h[k][0])
					}
					h[0], h[2] = h[2], h[0]
					mapping.Offset = new(big.Rat).Add(mapping.Offset, mapping.Scale)
					mapping.Scale = new(big.Rat).Neg(mapping.Scale)
				}
				native, e := newExactQuadraticUV(h, rationalEndpoints)
				if e != nil {
					return empty, e
				}
				start, end := edge.Start, edge.End
				if u.Reversed != f.Reversed {
					start, end = end, start
				}
				for side, vertex := range []int{start, end} {
					uv, _, e := native.EvaluateRational(big.NewRat(int64(side), 1))
					if e != nil {
						return empty, e
					}
					point, e := patch.EvaluateRational(ctx, uv[0], uv[1], o.Shell.Join.Face.Face.Mesh.Evaluation)
					if e != nil {
						return empty, e
					}
					expected := exactCorners[vertex]
					for axis := range expected {
						if expected[axis].Cmp(point.Point[axis]) != 0 {
							return empty, geom.ErrConvergence
						}
					}
					usedVertices[vertex] = true
				}
				nativeLoop = append(nativeLoop, native)
				originalTrims[fi] = append(originalTrims[fi], baseIndex+index)
				parameterMaps[fi] = append(parameterMaps[fi], mapping)
				uses[u.Edge] = append(uses[u.Edge], ownership{fi, baseIndex + j, start, end})
			}
			if li == 0 {
				faces[fi].Outer = nativeLoop
			} else {
				faces[fi].Holes = append(faces[fi].Holes, nativeLoop)
			}
			baseIndex += len(loop)
		}
	}
	var seams []QuadraticTrimStitchSeam
	junctions := make([][]QuadraticTrimSeamEndpoint, len(s.vertices))
	originalEdges := make([]int, 0, len(s.edges))
	originalVertices := make([]int, len(s.vertices))
	for edge, pair := range uses {
		if len(pair) != 2 || pair[0].face == pair[1].face || pair[0].start != pair[1].end || pair[0].end != pair[1].start {
			return empty, geom.ErrInvalid
		}
		si := len(seams)
		a, b := pair[0], pair[1]
		seams = append(seams, QuadraticTrimStitchSeam{FaceA: a.face, TrimA: a.trim, FaceB: b.face, TrimB: b.trim})
		junctions[a.start] = append(junctions[a.start], QuadraticTrimSeamEndpoint{Seam: si, End: 0})
		junctions[a.end] = append(junctions[a.end], QuadraticTrimSeamEndpoint{Seam: si, End: 1})
		originalEdges = append(originalEdges, edge)
	}
	for i, used := range usedVertices {
		if !used {
			return empty, geom.ErrInvalid
		}
		originalVertices[i] = i
	}
	shell, e := StitchEmbeddedExactQuadraticShellCertified(ctx, faces, seams, junctions, o.Shell)
	if e != nil {
		return empty, e
	}
	// Region certificates normalize each hole to CCW. Preserve the map from
	// that returned trim parameter back to the original stored curve.
	for fi, reversed := range shell.TrimReversed {
		for ti, reverse := range reversed {
			if reverse {
				m := &parameterMaps[fi][ti]
				m.Offset = new(big.Rat).Add(m.Offset, m.Scale)
				m.Scale = new(big.Rat).Neg(m.Scale)
			}
		}
	}
	shell.OriginalTrim = originalTrims
	return CertifiedSolidExactQuadraticMesh{CertifiedEmbeddedExactQuadraticShell: shell, OriginalEdges: originalEdges, OriginalVertices: originalVertices, ReparameterizedFaces: reflected, SourceParameters: parameterMaps, ModelEdgeUsesCertified: proofs, RationalCorners: rationalCorners}, nil
}
func solidSplineParameterMap(curve SplineUV) ExactTrimParameterMap {
	offset := ratInset(curve.Range.Min)
	scale := new(big.Rat).Sub(ratInset(curve.Range.Max), offset)
	if curve.Reversed {
		offset = ratInset(curve.Range.Max)
		scale.Neg(scale)
	}
	return ExactTrimParameterMap{Offset: offset, Scale: scale, Degree: curve.Curve.Degree()}
}
