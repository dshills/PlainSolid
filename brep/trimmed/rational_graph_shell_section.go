package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// RationalGraphShellSectionOptions limits the face/span certificates and
// total section pieces (default128, maximum1024).
type RationalGraphShellSectionOptions struct {
	Spans     PolynomialPlaneSpanOptions
	MaxPieces int
	// VerifyModelEdges independently isolates the complete contact set on
	// each intersected model edge, in addition to both owning UV proofs.
	// Limits apply per edge. Some proven coedges may lack a model descriptor
	// in this narrower optional verification domain.
	VerifyModelEdges bool
	EdgeContacts     geom.ComposedPlaneContactsOptions
}

// RationalGraphShellTerminal retains an original edge, its model-curve
// parameter, and an exact spatial point. The rationals are caller-owned.
type RationalGraphShellTerminal struct {
	Edge      int
	Parameter *big.Rat
	Point     [3]*big.Rat
}

// RationalGraphShellBranch traverses one original face. Pieces run from the
// first to the second terminal, in the original UV chart. The direction in
// model space is the oriented plane normal crossed with the outward shell
// normal; the planar material region is to the left in that plane.
type RationalGraphShellBranch struct {
	Face   int
	Pieces []PCurve
	Ends   [2]RationalGraphShellTerminal
}

// RationalGraphShellSection contains the entire certified section, in
// directed wires of branch indices. An empty result can be complete.
type RationalGraphShellSection struct {
	Branches []RationalGraphShellBranch
	Wires    [][]int
	Complete bool
}

// SectionRationalGraphSplineShell certifies a plane section of one outward
// embedded rectangular spline shell. Each face is either strictly separated,
// intersected in an affine line, or has one complete transverse graph in one
// original chart direction. Coplanar, edge and vertex contacts, and faces
// outside the graph family fail atomically. Affine line sections can retain
// rational endpoints distinct from their float64 display coordinates.
// Open face sections join only on
// the same original model edge, with exact parameter and model point equality.
// Wires retain orientation and do not imply a cap or a solid Boolean.
func (s Solid) SectionRationalGraphSplineShell(ctx context.Context, points [3]geom.Vec3, o RationalGraphShellSectionOptions) (RationalGraphShellSection, error) {
	var zero RationalGraphShellSection
	if o.MaxPieces == 0 {
		o.MaxPieces = 128
	}
	if ctx == nil || o.MaxPieces < 1 || o.MaxPieces > 1024 || o.Spans.MaxSpans < 0 || o.Spans.MaxSpans > 128 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, p := range points {
		if !p.IsFinite() {
			return zero, geom.ErrInvalid
		}
	}
	plane, e := polynomialPlaneFromPoints(points)
	if e != nil {
		return zero, e
	}
	var faces []splineEmbeddingFace
	if r, e := s.validateSplineRectangleEmbedding(ctx, o.Spans.Faces, &faces); e != nil || !r.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return zero, e
	}
	var out RationalGraphShellSection
	count := 0
	for id, f := range faces {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		h, e := geom.CertifySurfaceSupportingPlane(ctx, f.patch, points[0], points[1], points[2], o.Spans.Faces.Plane)
		if e == nil {
			if !h.Complete || h.Coincident || h.HalfspaceSign == 0 || h.SideMask != 0 || h.CornerMask != 0 {
				return zero, fmt.Errorf("trimmed: plane contact on face %d is not a transverse graph: %w", id, geom.ErrConvergence)
			}
			continue
		}
		if !errors.Is(e, geom.ErrConvergence) {
			return zero, e
		}
		b, e := s.rationalGraphShellFaceBranch(ctx, id, f, plane, o.Spans)
		if e != nil {
			return zero, fmt.Errorf("trimmed: section face %d: %w", id, e)
		}
		count += len(b.Pieces)
		if count > o.MaxPieces {
			return zero, geom.ErrConvergence
		}
		out.Branches = append(out.Branches, b)
	}
	out.Wires, e = joinRationalGraphShellBranches(ctx, out.Branches)
	if e != nil {
		return zero, e
	}
	if o.VerifyModelEdges {
		if e = s.verifyRationalGraphShellModelEdges(ctx, out.Branches, plane, o); e != nil {
			return zero, e
		}
	}
	out.Complete = true
	return out, nil
}

func (s Solid) verifyRationalGraphShellModelEdges(ctx context.Context, branches []RationalGraphShellBranch, plane polynomialPlaneEquation, o RationalGraphShellSectionOptions) error {
	raw := [4]*big.Rat{plane.normal[0], plane.normal[1], plane.normal[2], new(big.Rat).Neg(plane.offset)}
	for _, b := range branches {
		// Each original cut edge occurs once as a branch end and once as a
		// branch start. The directed graph has already proved their equality.
		x := b.Ends[1]
		if e := ctx.Err(); e != nil {
			return e
		}
		if e := verifyRationalGraphModelTerminal(ctx, s.edges[x.Edge], x, raw, o.Spans.Faces.Patches, o.EdgeContacts); e != nil {
			return fmt.Errorf("trimmed: section edge %d: %w", x.Edge, e)
		}
	}
	return nil
}

func (s Solid) rationalGraphShellFaceBranch(ctx context.Context, id int, f splineEmbeddingFace, plane polynomialPlaneEquation, o PolynomialPlaneSpanOptions) (RationalGraphShellBranch, error) {
	chart, e := geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{
		MaxOperations: o.Faces.Patches.MaxOperations, MaxCoefficientBits: o.Faces.Patches.MaxCoefficientBits,
	})
	if e == nil {
		return s.affineShellPlaneBranch(ctx, id, chart, plane)
	}
	if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) {
		return RationalGraphShellBranch{}, e
	}
	var p *SplineGraphPlaneSection
	for axis := range 2 {
		p, e = certifyRationalGraphPlaneFaceSection(ctx, s, id, plane, axis, o)
		if e == nil {
			break
		}
		if !errors.Is(e, geom.ErrConvergence) {
			return RationalGraphShellBranch{}, e
		}
	}
	if e != nil {
		return RationalGraphShellBranch{}, e
	}
	b := RationalGraphShellBranch{Face: id, Pieces: p.Pieces()}
	edges, parameters := p.Edges(), p.Parameters()
	for i := range b.Ends {
		piece := b.Pieces[0]
		if i == 1 {
			piece = b.Pieces[len(b.Pieces)-1]
		}
		exact, _, e := polynomialPlaneEndpoints(piece)
		if e != nil {
			return RationalGraphShellBranch{}, e
		}
		j, e := f.patch.EvaluateRational(ctx, exact[i][0], exact[i][1], geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.Faces.Patches.MaxOperations, MaxCoefficientBits: o.Faces.Patches.MaxCoefficientBits,
		})
		if e != nil {
			return RationalGraphShellBranch{}, e
		}
		b.Ends[i] = RationalGraphShellTerminal{edges[i], parameters[i], j.Point}
	}
	forward := p.PlaneSideSign()
	if (p.FreeAxis() == 1) != s.faces[id].Reversed {
		forward = -forward
	}
	if forward < 0 {
		b.Ends[0], b.Ends[1] = b.Ends[1], b.Ends[0]
		for i, j := 0, len(b.Pieces)-1; i <= j; i, j = i+1, j-1 {
			a, c := b.Pieces[i], b.Pieces[j]
			b.Pieces[i], b.Pieces[j] = reversePolynomialPlaneCurve(c), reversePolynomialPlaneCurve(a)
		}
	}
	return b, nil
}

func (s Solid) affineShellPlaneBranch(ctx context.Context, id int, chart *geom.AffineSurfaceChart, plane polynomialPlaneEquation) (RationalGraphShellBranch, error) {
	var zero RationalGraphShellBranch
	f := s.faces[id]
	uv := slabFaceUV(f)
	var heights [4]*big.Rat
	var points [4][3]*big.Rat
	for i, q := range uv {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		p, e := chart.EvalExact(q[0], q[1])
		if e != nil {
			return zero, e
		}
		points[i] = p
		heights[i] = new(big.Rat).Sub(plane.dot(p), plane.offset)
		if heights[i].Sign() == 0 {
			return zero, geom.ErrConvergence
		}
	}
	b := RationalGraphShellBranch{Face: id}
	var ends [2][2]*big.Rat
	for i, u := range f.Outer {
		j := (i + 1) % len(uv)
		a, c := heights[i], heights[j]
		if a.Sign() == c.Sign() {
			continue
		}
		t := new(big.Rat).Quo(a, new(big.Rat).Sub(a, c))
		// The section points out of the face at the terminal where the
		// oriented boundary residual goes from negative to positive.
		end := 0
		if (a.Sign()*plane.orientation < 0) != f.Reversed {
			end = 1
		}
		if ends[end][0] != nil {
			return zero, geom.ErrConvergence
		}
		blend := func(a, b *big.Rat) *big.Rat {
			return new(big.Rat).Add(a, new(big.Rat).Mul(t, new(big.Rat).Sub(b, a)))
		}
		for k := range ends[end] {
			ends[end][k] = blend(uv[i][k], uv[j][k])
		}
		var point [3]*big.Rat
		for k := range point {
			point[k] = blend(points[i][k], points[j][k])
		}
		if u.Reversed {
			t.Sub(big.NewRat(1, 1), t)
		}
		r := s.edges[u.Edge].Range
		b.Ends[end] = RationalGraphShellTerminal{Edge: u.Edge, Parameter: blend(ratInset(r.Min), ratInset(r.Max)), Point: point}
	}
	if ends[0][0] == nil || ends[1][0] == nil {
		return zero, geom.ErrConvergence
	}
	q, e := quadraticLineWithEndpoints(ends[0], ends[1], true)
	if e != nil {
		return zero, e
	}
	b.Pieces = []PCurve{q}
	return b, nil
}

func joinRationalGraphShellBranches(ctx context.Context, branches []RationalGraphShellBranch) ([][]int, error) {
	type terminal struct {
		branch, end int
	}
	pending := map[int]terminal{}
	resolved := map[int]bool{}
	next := make([]int, len(branches))
	for i, b := range branches {
		next[i] = -1
		for j, end := range b.Ends {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			if resolved[end.Edge] {
				return nil, geom.ErrConvergence
			}
			t, ok := pending[end.Edge]
			if !ok {
				pending[end.Edge] = terminal{i, j}
				continue
			}
			other := branches[t.branch].Ends[t.end]
			if i == t.branch || j == t.end || end.Parameter.Cmp(other.Parameter) != 0 {
				return nil, geom.ErrConvergence
			}
			for k := range end.Point {
				if end.Point[k].Cmp(other.Point[k]) != 0 {
					return nil, geom.ErrConvergence
				}
			}
			a, c := i, t.branch
			if j == 0 {
				a, c = c, a
			}
			next[a] = c
			delete(pending, end.Edge)
			resolved[end.Edge] = true
		}
	}
	if len(pending) != 0 {
		return nil, geom.ErrConvergence
	}
	var wires [][]int
	visited := make([]bool, len(branches))
	for i := range branches {
		if visited[i] {
			continue
		}
		j := i
		var wire []int
		for j >= 0 && !visited[j] {
			wire = append(wire, j)
			visited[j] = true
			j = next[j]
		}
		if j != i {
			return nil, geom.ErrConvergence
		}
		wires = append(wires, wire)
	}
	return wires, nil
}
