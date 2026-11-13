package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineSectionPiece is a certified segment with its two original support
// face indices. Reversed follows the section wire traversal rather than
// the segment's canonical intersection tangent. Reversing traversal means
// evaluating Segment at 1-t; the segment's paired trims remain unchanged.
type SplineSectionPiece struct {
	FaceA, FaceB int
	Segment      intersect.TubeSegment
	Reversed     bool
}

// SplineSectionWire is a cyclic sequence of exact section pieces. Each
// piece retains its paired p-curves and model point enclosures.
type SplineSectionWire struct {
	Pieces []SplineSectionPiece
}

// SplineShellSectionReport describes complete transverse sections of two
// embedded spline shells. FacePairs indexes Pairs in the same order. A
// partial report can retain local pair proofs but has no usable wire graph.
type SplineShellSectionReport struct {
	Wires     []SplineSectionWire
	FacePairs [][2]int
	Pairs     []intersect.SplinePairTrace
	Complete  bool
}

// SplineShellPartition contains one original shell with its boundary faces
// partitioned at the section. Faces and Edges map each original entity to
// its resulting fragments; original model edge fragments follow increasing
// original parameter. SectionEdges aligns with the other operand's row.
type SplineShellPartition struct {
	Solid        Solid
	Faces, Edges [][]int
	SectionEdges []int
}

// SplitSectionedSplineShells partitions both original boundaries without
// removing any material. Each supplied proposal and the complete section set
// are independently certified. It returns no partitions on failure.
func SplitSectionedSplineShells(ctx context.Context, a, b Solid, sections []SplineShellSection,
	o SplineShellOpenOptions) (out [2]SplineShellPartition, err error) {
	if ctx == nil || len(sections) == 0 || len(sections) > 64 {
		return out, geom.ErrInvalid
	}
	_, _, w, _, e := certifySectionedSplineShells(ctx, a, b, sections, o)
	if e != nil {
		return out, e
	}
	model, e := w.modelEndpoints()
	if e != nil {
		return out, e
	}
	if e := w.checkDistinctWireVertices(model, a, b); e != nil {
		return out, e
	}
	for operand, s := range [2]Solid{a, b} {
		p, e := w.splitShell(ctx, s, operand, model, o.Endpoints)
		if e != nil {
			return [2]SplineShellPartition{}, e
		}
		proof, e := p.solid.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.Endpoints.MaxOperations, MaxBits: o.Endpoints.MaxCoefficientBits})
		if e != nil || !proof.Complete {
			if e == nil {
				e = geom.ErrConvergence
			}
			return [2]SplineShellPartition{}, e
		}
		x := SplineShellPartition{Solid: p.solid, Faces: make([][]int, len(s.faces)), Edges: p.edges,
			SectionEdges: make([]int, len(model))}
		for i, f := range p.faces {
			x.Faces[f] = append(x.Faces[f], i)
		}
		seen := make([]bool, len(model))
		for i, j := range p.section {
			if seen[j] {
				return [2]SplineShellPartition{}, geom.ErrConvergence
			}
			seen[j] = true
			x.SectionEdges[j] = i
		}
		for _, ok := range seen {
			if !ok {
				return [2]SplineShellPartition{}, geom.ErrConvergence
			}
		}
		out[operand] = x
	}
	return out, nil
}

// SectionSplineShells returns the complete exact transverse wire graph
// for two embedded rectangular spline shells. Optional supplied Sections
// undergo independent pair coverage and endpoint verification. Without
// proposals, affine and plane/spline discovery is used on all possible
// pairs. Tangencies and coincidence remain unresolved. Strictly separated
// and nested shells produce an empty complete section set.
func SectionSplineShells(ctx context.Context, a, b Solid, o SplineShellCombineOptions) (SplineShellSectionReport, error) {
	var out SplineShellSectionReport
	if ctx == nil || len(o.Sections) != 0 && len(o.OpenSections) != 0 || !finite(o.VerticalQuadraticWarp) {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o.Open.verticalQuadraticWarp = o.VerticalQuadraticWarp
	if o.Open.Shells.Embedding == (SplineRectangleEmbeddingOptions{}) {
		o.Open.Shells.Embedding = o.Transverse.Shells.Embedding
	}
	if o.Open.Shells.Separation == (geom.SurfaceSurfaceIsolationOptions{}) {
		o.Open.Shells.Separation = o.Transverse.Shells.Separation
	}
	var ap, bp []splineEmbeddingFace
	for _, x := range []struct {
		s Solid
		p *[]splineEmbeddingFace
	}{{a, &ap}, {b, &bp}} {
		if len(x.s.faces) == 0 {
			continue
		}
		if _, e := x.s.validateSplineRectangleEmbedding(ctx, o.Open.Shells.Embedding, x.p); e != nil {
			return out, e
		}
	}
	sections := o.Sections
	if len(o.OpenSections) != 0 {
		sections = o.OpenSections
		for _, s := range sections {
			if s.Closed {
				return out, geom.ErrInvalid
			}
		}
	}
	if len(sections) == 0 {
		var pairs [][2]int
		for i, p := range ap {
			for j, q := range bp {
				e := proveSplinePatchPairDisjoint(ctx, p.patch, q.patch, o.Open.Shells.Separation)
				if e == nil {
					continue
				}
				if !isUnresolvedOpenSpline(e) {
					return out, e
				}
				pairs = append(pairs, [2]int{i, j})
			}
		}
		if len(pairs) == 0 {
			out.Complete = true
			return out, nil
		}
		var e error
		if !o.DisableAffineDiscovery && o.Open.verticalQuadraticWarp == 0 && o.DiscoverVerticalQuadraticWarp {
			k, e := discoverSplineShellVerticalQuadratic(ctx, ap, bp, o.Open.Trace.Seeds.Patches)
			if e != nil && !isUnresolvedOpenSpline(e) {
				return out, e
			}
			o.Open.verticalQuadraticWarp = k
		}
		if !o.DisablePlaneDiscovery || !o.DisableAffineDiscovery || o.EnableGeneralTubeDiscovery {
			sections, _, e = discoverSplineShellSections(ctx, a, b, ap, bp, pairs, o.Open,
				!o.DisableAffineDiscovery, !o.DisablePlaneDiscovery, o.EnableGeneralTubeDiscovery)
		} else {
			e = geom.ErrConvergence
		}
		if e != nil {
			return out, e
		}
	}
	if len(sections) > 64 {
		return out, geom.ErrInvalid
	}
	var h SplineShellOpenHistory
	curves, e := certifyOpenSplinePairs(ctx, a, b, ap, bp, sections, o.Open, &h)
	out.FacePairs, out.Pairs = h.FacePairs, h.Pairs
	if e != nil {
		return out, e
	}
	w, e := prepareSplineShellWire(ctx, a, b, ap, bp, sections, curves, o.Open.Endpoints)
	if e != nil {
		return out, e
	}
	// Use the same represented vertex collision check as Boolean assembly.
	model, e := w.modelEndpoints()
	if e != nil {
		return out, e
	}
	if e := w.checkDistinctWireVertices(model, a, b); e != nil {
		return out, e
	}
	wires, e := w.sectionWires(sections, curves)
	if e != nil {
		return out, e
	}
	out.Wires, out.Complete = wires, true
	return out, nil
}

func (w openSplineWire) sectionWires(sections []SplineShellSection, curves [][]intersect.TubeSegment) ([]SplineSectionWire, error) {
	var wires []SplineSectionWire
	for i, s := range sections {
		if !s.Closed {
			continue
		}
		wire := SplineSectionWire{}
		for _, c := range curves[i] {
			wire.Pieces = append(wire.Pieces, SplineSectionPiece{s.FaceA, s.FaceB, c, false})
		}
		wires = append(wires, wire)
	}
	seen := make([]bool, len(w.branches))
	for start := range w.branches {
		if seen[start] {
			continue
		}
		wire := SplineSectionWire{}
		for next := 2 * start; ; {
			if next < 0 || next/2 >= len(seen) || seen[next/2] {
				return nil, fmt.Errorf("trimmed: unclosed spline wire: %w", geom.ErrConvergence)
			}
			b := w.branches[next/2]
			seen[next/2] = true
			rev := next%2 == 1
			for i := range b.segments {
				j := i
				if rev {
					j = len(b.segments) - 1 - i
				}
				wire.Pieces = append(wire.Pieces, SplineSectionPiece{b.faces[0], b.faces[1], b.segments[j], rev})
			}
			next = w.joins[next/2][(next%2)^1]
			if next == 2*start {
				break
			}
		}
		wires = append(wires, wire)
	}
	return wires, nil
}
