package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicShellCoedgeOccurrence preserves one actual original face use.
// Parameter is normalized in its stored UVStart-to-UVEnd direction.
// ModelEvent retains canonical model-edge traversal; Reversed gives the exact
// affine conversion, and FaceReversed separately records outward traversal.
type AlgebraicShellCoedgeOccurrence struct {
	Face, LocalEdge        int
	Reversed, FaceReversed bool
	Parameter              *geom.ExactAlgebraicRoot
	ModelEvent             *geom.ExactComposedCurveEvent
}

// AlgebraicShellBoundaryOwner binds one original model edge to all of its
// incident coedges. EdgeParameter follows the stored model curve's [0,1] range.
type AlgebraicShellBoundaryOwner struct {
	Operand, Face, Edge int
	Side                geom.SurfaceEdge
	EdgeParameter       *geom.ExactAlgebraicRoot
	ModelEvent          *geom.ExactComposedCurveEvent
	Incidents           [2]AlgebraicShellCoedgeOccurrence
}
type AlgebraicShellPairedTerminal struct {
	Owners [2]AlgebraicShellBoundaryOwner
}
type AlgebraicShellOwnerBranch struct {
	Branch, Segments int
	Closed           bool
	Ends             [2]AlgebraicShellPairedTerminal
}

// AlgebraicShellFacePairOwners accounts for the complete selected face-pair
// section and every incident original edge use at its paired terminals. It is
// not a complete section of both shells: neighboring face pairs may contain
// additional curves, which must be independently discovered before assembly.
type AlgebraicShellFacePairOwners struct {
	sources  [2]Solid
	faces    [2]int
	branches []AlgebraicShellOwnerBranch
	work     int
}

func (r *AlgebraicShellFacePairOwners) Faces() [2]int {
	if r == nil {
		return [2]int{}
	}
	return r.faces
}
func (r *AlgebraicShellFacePairOwners) Branches() []AlgebraicShellOwnerBranch {
	if r == nil {
		return nil
	}
	return append([]AlgebraicShellOwnerBranch(nil), r.branches...)
}

// ExactOperations reports conservatively charged owner-correspondence work.
// Embedding, trace, and terminal-ledger stages retain their separate limits.
func (r *AlgebraicShellFacePairOwners) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}

type AlgebraicShellFacePairOwnerOptions struct {
	Embedding SplineRectangleEmbeddingOptions
	Trace     intersect.SplinePairTraceOptions
	Endpoints geom.TubeAlgebraicGraphEndpointOptions
	// MaxWork bounds cumulative owner-edge correspondence work separately from
	// the explicit embedding, trace and endpoint-ledger proof stages. Default256M.
	MaxWork int
}

// SectionSplineShellFacePairAlgebraicOwners reconstructs every certificate
// from the actual stored shells. It never accepts public Complete flags or an
// asserted incidence table. Each terminal edge must have exactly two distinct
// original face uses with opposite canonical edge traversal. Corners and
// missing/extra incidents reject atomically. Legacy shell readers are unchanged.
func SectionSplineShellFacePairAlgebraicOwners(ctx context.Context, a, b Solid, faces [2]int, o AlgebraicShellFacePairOwnerOptions) (*AlgebraicShellFacePairOwners, error) {
	if ctx == nil || o.MaxWork < 0 || o.MaxWork > 1000000000 {
		return nil, geom.ErrInvalid
	}
	limit := o.MaxWork
	if limit == 0 {
		limit = 256000000
	}
	opts := o.Endpoints
	opts.MaxOperations = limit
	budget, e := newPairedTerminalBudget(opts)
	if e != nil {
		return nil, e
	}
	solids := [2]Solid{a, b}
	var charts [2][]splineEmbeddingFace
	var patches [2]intersect.Patch
	for owner, s := range solids {
		if faces[owner] < 0 || faces[owner] >= len(s.faces) {
			return nil, geom.ErrInvalid
		}
		if _, e = s.validateSplineRectangleEmbedding(ctx, o.Embedding, &charts[owner]); e != nil {
			return nil, fmt.Errorf("trimmed: owner shell %d embedding: %w", owner, e)
		}
		patches[owner], e = algebraicShellTracePatch(s.faces[faces[owner]], charts[owner][faces[owner]].patch.Domain())
		if e != nil {
			return nil, e
		}
	}
	section, e := SectionSplinePairAlgebraicOwners(ctx, patches[0], patches[1], nil, o.Trace, o.Endpoints)
	if e != nil {
		return nil, fmt.Errorf("trimmed: paired face ledger: %w", e)
	}
	if !section.Complete {
		return nil, geom.ErrConvergence
	}
	result := &AlgebraicShellFacePairOwners{faces: faces, sources: solids}
	open := 0
	for branch, curve := range section.Trace.Curves {
		row := AlgebraicShellOwnerBranch{Branch: branch, Segments: len(curve), Closed: section.Trace.BranchGraph.Branches[branch].Closed}
		if !row.Closed {
			for end, terminal := range section.Open[open].Ends {
				for owner, occurrence := range terminal.Owners() {
					bound, e := bindPairedShellEdge(ctx, solids[owner], faces[owner], charts[owner][faces[owner]], occurrence, terminal.ModelEvent(), budget)
					if e != nil {
						return nil, fmt.Errorf("trimmed: branch %d end %d owner %d: %w", branch, end, owner, e)
					}
					row.Ends[end].Owners[owner] = bound
				}
			}
			open++
		}
		result.branches = append(result.branches, row)
	}
	if open != len(section.Open) {
		return nil, geom.ErrConvergence
	}
	result.work = budget.used
	return result, nil
}

func bindPairedShellEdge(ctx context.Context, s Solid, face int, chart splineEmbeddingFace, owner AlgebraicSplineBoundaryOwner, canonical *geom.ExactComposedCurveEvent, budget *pairedTerminalBudget) (AlgebraicShellBoundaryOwner, error) {
	empty := AlgebraicShellBoundaryOwner{}
	if owner.Side > geom.SurfaceEdgeVMax || owner.Parameter == nil || canonical == nil {
		return empty, geom.ErrInvalid
	}
	edge := chart.edge[owner.Side]
	axis := 1 - int(owner.Side)/2
	var selected *Use
	for i := range s.faces[face].Outer {
		u := &s.faces[face].Outer[i]
		if u.Edge == edge {
			if selected != nil {
				return empty, geom.ErrConvergence
			}
			selected = u
		}
	}
	if selected == nil || selected.Curve != nil {
		return empty, geom.ErrConvergence
	}
	p, q := selected.UVStart, selected.UVEnd
	if selected.Reversed {
		p, q = q, p
	}
	start, finish := p.X, q.X
	if axis == 1 {
		start, finish = p.Y, q.Y
	}
	lo, width := ratInset(start), new(big.Rat).Sub(ratInset(finish), ratInset(start))
	if width.Sign() == 0 {
		return empty, geom.ErrConvergence
	}
	rootOptions, e := budget.rootOptions()
	if e != nil {
		return empty, e
	}
	root, n, e := owner.Parameter.AffineMap(ctx, new(big.Rat).Neg(new(big.Rat).Quo(lo, width)), new(big.Rat).Inv(width), rootOptions)
	if e != nil {
		return empty, e
	}
	if e = budget.debit(n); e != nil {
		return empty, e
	}
	// Restrict the opaque continuous-comparison helper to straight original
	// iso uses. In this route its two comparisons, iso constructions/restrictions,
	// optional support placements and three support extractions use fewer than
	// 32 separately bounded arithmetic stages. Reserve all 32 allowances.
	modelCurve, ok := s.edges[edge].Curve.(surfaceTrimCurve)
	if !ok || modelCurve.curve != nil || (modelCurve.a.X != modelCurve.b.X && modelCurve.a.Y != modelCurve.b.Y) {
		return empty, geom.ErrConvergence
	}
	for _, f := range s.faces {
		for _, ring := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range ring {
				if u.Edge == edge && (u.Curve != nil || (u.UVStart.X != u.UVEnd.X && u.UVStart.Y != u.UVEnd.Y)) {
					return empty, geom.ErrConvergence
				}
			}
		}
	}
	const allowance = 250000
	if e = budget.debit(32 * allowance); e != nil {
		return empty, e
	}
	composition := budget.options.SurfaceCurveAgreementOptions
	composition.MaxOperations = allowance
	model, incidents, e := algebraicSeamSource(ctx, s, edge, composition)
	if e != nil {
		return empty, fmt.Errorf("edge %d original source: %w", edge, e)
	}
	if (incidents[0].use.Reversed != s.faces[incidents[0].face].Reversed) == (incidents[1].use.Reversed != s.faces[incidents[1].face].Reversed) {
		return empty, geom.ErrConvergence
	}
	result := AlgebraicShellBoundaryOwner{Operand: owner.Operand, Face: face, Edge: edge, Side: owner.Side, EdgeParameter: root}
	result.ModelEvent, e = bindShellOwnerImage(ctx, model, root, canonical, budget)
	if e != nil {
		return empty, fmt.Errorf("edge %d model binding: %w", edge, e)
	}
	selectedCount := 0
	for i, incident := range incidents {
		if incident.face == face {
			selectedCount++
		}
		event, e := bindShellOwnerImage(ctx, incident, root, canonical, budget)
		if e != nil {
			return empty, e
		}
		parameter := root
		if incident.use.Reversed {
			rootOptions, e = budget.rootOptions()
			if e != nil {
				return empty, e
			}
			parameter, n, e = root.AffineMap(ctx, big.NewRat(1, 1), big.NewRat(-1, 1), rootOptions)
			if e != nil {
				return empty, e
			}
			if e = budget.debit(n); e != nil {
				return empty, e
			}
		}
		local := -1
		for j, u := range s.faces[incident.face].Outer {
			if u.Edge == edge {
				if local >= 0 {
					return empty, geom.ErrConvergence
				}
				local = j
			}
		}
		if local < 0 {
			return empty, geom.ErrConvergence
		}
		result.Incidents[i] = AlgebraicShellCoedgeOccurrence{Face: incident.face, LocalEdge: local, Reversed: incident.use.Reversed, FaceReversed: s.faces[incident.face].Reversed, Parameter: parameter, ModelEvent: event}
	}
	if selectedCount != 1 {
		return empty, geom.ErrConvergence
	}
	return result, nil
}
func bindShellOwnerImage(ctx context.Context, source algebraicSeamOwner, root *geom.ExactAlgebraicRoot, canonical *geom.ExactComposedCurveEvent, budget *pairedTerminalBudget) (*geom.ExactComposedCurveEvent, error) {
	child, e := budget.endpointOptions()
	if e != nil {
		return nil, e
	}
	uv, e := source.curve.modelParameterCurve(ctx, child.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	if e = budget.debit(uv.ExactOperations()); e != nil {
		return nil, e
	}
	child, e = budget.endpointOptions()
	if e != nil {
		return nil, e
	}
	cells, e := source.patch.BuildSurfaceCurveCompositionCells(ctx, uv, child.SurfaceCurveAgreementOptions)
	if e != nil {
		return nil, e
	}
	if e = budget.debit(cells.ExactOperations); e != nil {
		return nil, e
	}
	if !cells.Complete {
		return nil, geom.ErrConvergence
	}
	var event *geom.ExactComposedCurveEvent
	for _, cell := range cells.Cells {
		opts, e := budget.rootOptions()
		if e != nil {
			return nil, e
		}
		if opts.MaxOperations > 100000 {
			opts.MaxOperations = 100000
		}
		if e = budget.debit(opts.MaxOperations); e != nil {
			return nil, e
		}
		candidate, err := cell.BindRoot(ctx, root, opts)
		if errors.Is(err, geom.ErrDomain) {
			continue
		}
		if err != nil {
			return nil, err
		}
		opts, e = budget.rootOptions()
		if e != nil {
			return nil, e
		}
		equal, n, e := candidate.CompareModelPoint(ctx, canonical, opts)
		if e != nil {
			return nil, e
		}
		if e = budget.debit(n); e != nil {
			return nil, e
		}
		if !equal {
			return nil, geom.ErrConvergence
		}
		event = candidate
	}
	if event == nil {
		return nil, geom.ErrConvergence
	}
	return event, nil
}
