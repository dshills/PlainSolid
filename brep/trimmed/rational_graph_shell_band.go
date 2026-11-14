package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

type RationalGraphParallelBandOptions struct {
	Sections RationalGraphShellSectionOptions
	Caps     RationalGraphSectionCapOptions
}

// RationalGraphParallelBand retains the material enclosed by two certified
// parallel sections. CapFaces and CapEdges follow input plane order.
type RationalGraphParallelBand struct {
	Solid                    Solid
	Sections                 RationalGraphParallelSections
	SourceFaces, SourceEdges [][]int
	CapFaces, CapEdges       [2][]int
}

// ClipParallelRationalGraphSplineShell retains the part of an original
// rectangular spline shell between two distinct parallel planes. A crossed
// face must either be affine or contain two certified graph branches in the
// same original chart direction and matching knot intervals. Faces crossed by
// only one plane use that certified halfspace. The caps must be authored
// affine charts in their respective planes. Vertices must be exactly binary
// unless the quadratic RationalEndpoints path is selected. A section outside
// the shell or an unresolved configuration rejects atomically.
func (s Solid) ClipParallelRationalGraphSplineShell(ctx context.Context, points [2][3]geom.Vec3, caps [2]geom.Surface, domains [2]geom.ParameterRectangle, o RationalGraphParallelBandOptions) (RationalGraphParallelBand, error) {
	var zero RationalGraphParallelBand
	if caps[0] == nil || caps[1] == nil || !domains[0].Valid() || !domains[1].Valid() {
		return zero, geom.ErrInvalid
	}
	so := o.Sections
	if so.Regions == nil {
		so.Regions = &RationalGraphShellRegionOptions{}
	}
	sections, faces, planes, signs, e := s.discoverParallelRationalGraphSplineShell(ctx, points, so)
	if e != nil {
		return zero, e
	}
	keep := [2]int{}
	keep[sections.Lower] = planes[sections.Lower].orientation
	keep[1-sections.Lower] = -planes[1-sections.Lower].orientation
	out, e := s.assembleRationalGraphBand(ctx, sections.Sections, faces, planes, signs, keep, caps, domains, o)
	if e == nil {
		out.Sections = sections
	}
	return out, e
}

// assembleRationalGraphBand consumes complete sections already proved
// disjoint on the entire stock. The retained side of each plane is expressed
// relative to its original ordered points.
func (s Solid) assembleRationalGraphBand(ctx context.Context, sections [2]RationalGraphShellSection, faces []splineEmbeddingFace,
	planes [2]polynomialPlaneEquation, signs [2][]int, keep [2]int, caps [2]geom.Surface,
	domains [2]geom.ParameterRectangle, o RationalGraphParallelBandOptions) (RationalGraphParallelBand, error) {
	var zero RationalGraphParallelBand
	var e error
	so := o.Sections
	lookup := [2]map[int]RationalGraphShellBranch{}
	var capParts [2][]RationalGraphSectionCap
	for i, sec := range sections {
		if len(sec.Branches) == 0 {
			return zero, geom.ErrConvergence
		}
		lookup[i] = make(map[int]RationalGraphShellBranch, len(sec.Branches))
		for _, b := range sec.Branches {
			lookup[i][b.Face] = b
		}
		capParts[i], e = sec.Regions.BuildCaps(ctx, caps[i], domains[i], o.Caps)
		if e != nil {
			return zero, e
		}
	}
	out := RationalGraphParallelBand{SourceFaces: make([][]int, len(faces))}
	var parts []SewFace
	var owners [][]int
	for id, f := range faces {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		if signs[0][id] == -keep[0] || signs[1][id] == -keep[1] {
			continue
		}
		b0, c0 := lookup[0][id]
		b1, c1 := lookup[1][id]
		var p SewFace
		var old []int
		switch {
		case !c0 && !c1:
			p, old = s.splineSewFaceMapped(s.faces[id], nil)
			if o.Caps.RationalEndpoints {
				p, e = rationalQuadraticSewFace(ctx, s.faces[id], f.patch, slabFaceUV(s.faces[id]), so.Spans.Faces.Patches)
				old = nil
			}
		case c0 && !c1:
			p, e = splitRationalGraphFaceFragment(ctx, s.faces[id], f.patch, b0, planes[0], keep[0], so.Spans.Faces, o.Caps.RationalEndpoints)
		case !c0 && c1:
			p, e = splitRationalGraphFaceFragment(ctx, s.faces[id], f.patch, b1, planes[1], keep[1], so.Spans.Faces, o.Caps.RationalEndpoints)
		default:
			p, e = rationalGraphBandFace(ctx, s.faces[id], f.patch, [2]RationalGraphShellBranch{b0, b1}, planes, keep, so.Spans.Faces, o.Caps.RationalEndpoints)
		}
		if e != nil {
			return zero, fmt.Errorf("trimmed: plane band face %d: %w", id, e)
		}
		if old == nil {
			old, e = rationalGraphFragmentEdgeSources(s.faces[id], p)
			if e != nil {
				return zero, e
			}
		}
		out.SourceFaces[id] = []int{len(parts)}
		parts = append(parts, p)
		owners = append(owners, old)
	}
	for i, row := range capParts {
		for _, c := range row {
			p := c.Boundary
			if keep[i] > 0 {
				p.Face.Reversed = !p.Face.Reversed
			}
			out.CapFaces[i] = append(out.CapFaces[i], len(parts))
			parts = append(parts, p)
		}
	}
	if o.Caps.RationalEndpoints {
		if e = proveRationalSplitVertexIdentity(ctx, parts); e != nil {
			return zero, e
		}
	}
	solid, h, e := Sew(ctx, parts, SewOptions{Tolerance: s.tolerance, MaxFaces: len(parts), Proof: so.Spans.Faces.EdgeAgreement})
	if e != nil {
		return zero, e
	}
	for i, row := range out.CapFaces {
		for _, f := range row {
			out.CapEdges[i] = append(out.CapEdges[i], h.Edges[f]...)
		}
	}
	out.SourceEdges, e = classifyRationalGraphSewnEdges(solid, h, owners, len(s.edges), out.CapEdges[:])
	if e != nil {
		return zero, e
	}
	out.Solid = solid
	return out, nil
}

func rationalGraphBandFace(ctx context.Context, face Face, patch *geom.SurfacePatch, branches [2]RationalGraphShellBranch, planes [2]polynomialPlaneEquation, keep [2]int, o SplineRectangleEmbeddingOptions, rationalEndpoints bool) (SewFace, error) {
	var uv [][2]*big.Rat
	if branches[0].freeAxis < 0 && branches[1].freeAxis < 0 {
		uv = slabFaceUV(face)
		for i, p := range planes {
			var clipped [][2]*big.Rat
			var h []*big.Rat
			for _, x := range uv {
				j, e := patch.EvaluateRational(ctx, x[0], x[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
				if e != nil {
					return SewFace{}, e
				}
				h = append(h, new(big.Rat).Sub(p.dot(j.Point), p.offset))
			}
			for j, a := range uv {
				k := (j + 1) % len(uv)
				if h[j].Sign()*p.orientation == keep[i] {
					clipped = append(clipped, a)
				}
				if h[j].Sign()*h[k].Sign() < 0 {
					t := new(big.Rat).Quo(h[j], new(big.Rat).Sub(h[j], h[k]))
					var z [2]*big.Rat
					for axis := range z {
						z[axis] = new(big.Rat).Add(a[axis], new(big.Rat).Mul(t, new(big.Rat).Sub(uv[k][axis], a[axis])))
					}
					clipped = append(clipped, z)
				}
			}
			uv = clipped
		}
	} else {
		free := branches[0].freeAxis
		if free < 0 || free != branches[1].freeAxis || len(branches[0].Pieces) != len(branches[1].Pieces) {
			return SewFace{}, geom.ErrConvergence
		}
		var chain [2][][2]*big.Rat
		lower := -1
		for i, b := range branches {
			chain[i] = make([][2]*big.Rat, len(b.Pieces)+1)
			for j, q := range b.Pieces {
				p, _, e := graphSectionCapEndpoints(q, rationalEndpoints)
				if e != nil {
					return SewFace{}, e
				}
				chain[i][j], chain[i][j+1] = p[0], p[1]
			}
			if b.reversed {
				reverseGraphUVChain(chain[i])
			}
			if b.positive == keep[i] {
				if lower >= 0 {
					return SewFace{}, geom.ErrConvergence
				}
				lower = i
			}
		}
		if lower < 0 {
			return SewFace{}, geom.ErrConvergence
		}
		a, b := chain[lower], chain[1-lower]
		for i, p := range a {
			if p[free].Cmp(b[i][free]) != 0 || p[1-free].Cmp(b[i][1-free]) >= 0 {
				return SewFace{}, geom.ErrConvergence
			}
		}
		// No retained source point can be on both sections by the caller's
		// complete separation proof. Continuous graph branches cannot change
		// order between these independently ordered exact knot endpoints.
		if free == 1 {
			a, b = b, a
		}
		reverseGraphUVChain(b)
		uv = append(a, b...)
	}
	var part SewFace
	var e error
	if rationalEndpoints {
		part, e = rationalQuadraticSewFace(ctx, face, patch, uv, o.Patches)
	} else {
		part, e = slabSewFace(ctx, face, patch, uv, o)
	}
	if e != nil {
		return SewFace{}, e
	}
	if e = installRationalGraphFragmentTrims(ctx, &part, append(append([]PCurve(nil), branches[0].Pieces...), branches[1].Pieces...), rationalEndpoints); e != nil {
		return SewFace{}, e
	}
	return part, nil
}
