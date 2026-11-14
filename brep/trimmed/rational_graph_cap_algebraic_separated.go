package trimmed

import (
	"context"
	"errors"

	"plainsolid/geom"
)

// separatedAlgebraicCap keeps the entire original section. The other
// plane is proved strictly outside the control hull of every loop; no
// fictional seam or reconstructed cap edge is introduced.
func (s Solid) separatedAlgebraicCap(ctx context.Context, points, keep [3]geom.Vec3,
	section RationalGraphShellSectionOptions, o RationalGraphShellRegionOptions) (AlgebraicGraphCapClip, error) {
	var zero AlgebraicGraphCapClip
	for _, p := range keep {
		if !p.IsFinite() {
			return zero, geom.ErrInvalid
		}
	}
	if section.Regions == nil {
		section.Regions = &o
	}
	plane, e := polynomialPlaneFromPoints(keep)
	if e != nil {
		return zero, e
	}
	src, e := s.SectionRationalGraphSplineShell(ctx, points, section)
	if e != nil {
		return zero, e
	}
	// This test is an optimization gate only. When the strict condition
	// is unproved, the regular crossing path may still qualify.
	r := src.Regions
	if e := r.proveStrictCapHalfspace(ctx, plane, -1); e != nil {
		if errors.Is(e, geom.ErrConvergence) {
			return zero, geom.ErrUnrepresentable
		}
		return zero, e
	}
	opts, e := o.Topology.WithDefaults()
	if e != nil {
		return zero, e
	}
	if opts.MaxEdges > 4096 || opts.MaxBits < 64 {
		return zero, geom.ErrInvalid
	}
	b := capClipBudget{ctx, opts.MaxWork, opts.MaxSubdivisions, opts.MaxBits}
	out := AlgebraicGraphCapClip{
		Section: AlgebraicGraphCapSection{ProjectionAxes: orientedRationalGraphProjection(r.plane)},
		regions: r, cut: plane, keepSign: -plane.orientation, separated: true,
		recipe: &algebraicGraphCapRecipe{s, points, keep, false, section.ComposeSectionCurves, src.Branches},
	}
	count := 0
	for wi, wire := range r.wires {
		var row []AlgebraicGraphCapUse
		for ui, p := range wire {
			count++
			if count > opts.MaxEdges {
				return zero, geom.ErrConvergence
			}
			if e := b.charge(len(p.control)*12, 0); e != nil {
				return zero, e
			}
			cell, e := r.capClipCell(p.control, &b)
			if e != nil {
				return zero, e
			}
			lo, _ := cell.EndpointRoot(0)
			hi, _ := cell.EndpointRoot(1)
			param, e := capClipSourceParameters(p, [2]int{-1, -1}, [2]*geom.ExactAlgebraicRoot{lo, hi}, out.Section, &b)
			if e != nil {
				return zero, e
			}
			row = append(row, AlgebraicGraphCapUse{Cell: cell, EndpointContacts: [2]int{-1, -1}, InputWire: wi, InputPiece: ui,
				Seam: -1, Source: p.source, SourceParameters: param})
		}
		out.wires = append(out.wires, row)
		out.Wires = append(out.Wires, append([]AlgebraicGraphCapUse(nil), row...))
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations, out.Subdivisions = opts.MaxWork-b.work, opts.MaxSubdivisions-b.subdiv
	return out, nil
}
