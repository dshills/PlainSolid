package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// discoverSplineShellSections can combine affine line proposals with curved
// plane section proposals. Each proposed pair is still subject to independent
// coverage, exact edge reconciliation and shell assembly.
func discoverSplineShellSections(ctx context.Context, a, b Solid, ap, bp []splineEmbeddingFace,
	pairs [][2]int, o SplineShellOpenOptions, affine, plane, general bool) ([]SplineShellOpenSection, bool, error) {
	if len(pairs) == 0 || len(pairs) > 64 {
		return nil, false, geom.ErrConvergence
	}
	var sections []SplineShellOpenSection
	var deferred [][2]int
	curved := false
	for _, pair := range pairs {
		if e := ctx.Err(); e != nil {
			return nil, false, e
		}
		if affine {
			q, e := discoverAffineSplineOpenSections(ctx, a, b, ap, bp, [][2]int{pair}, o)
			if e == nil {
				curved = curved || o.verticalQuadraticWarp != 0
				sections = append(sections, q...)
				if len(sections) > 64 {
					return nil, false, geom.ErrConvergence
				}
				continue
			}
			if !isUnresolvedOpenSpline(e) {
				return nil, false, e
			}
		}
		p, e := intersectSplineFace(a.faces[pair[0]], ap[pair[0]].patch.Domain())
		if e != nil {
			return nil, false, e
		}
		q, e := intersectSplineFace(b.faces[pair[1]], bp[pair[1]].patch.Domain())
		if e != nil {
			return nil, false, e
		}
		var r []SplineShellOpenSection
		if plane {
			r, e = proposePlaneSplineSections(ctx, p, q, pair, o.Trace)
		} else {
			e = geom.ErrConvergence
		}
		if e != nil && general && isUnresolvedOpenSpline(e) {
			r, e = proposeGeneralSplineSections(ctx, p, q, pair, o)
		}
		if e != nil {
			if !isUnresolvedOpenSpline(e) {
				return nil, false, fmt.Errorf("trimmed: discover A:%d B:%d: %w", pair[0], pair[1], e)
			}
			// At a simultaneous edge event, two adjacent face pairs may
			// contain only that exact point. Later global certification
			// independently proves those exclusions using the anchored
			// branch endpoints. Other missing pairs still fail there.
			deferred = append(deferred, pair)
			continue
		}
		curved = true
		sections = append(sections, r...)
		if len(sections) > 64 {
			return nil, false, geom.ErrConvergence
		}
	}
	if len(sections) == 0 && len(deferred) != 0 {
		p := deferred[0]
		return nil, false, fmt.Errorf("trimmed: discover A:%d B:%d: %w", p[0], p[1], geom.ErrConvergence)
	}
	return sections, curved, nil
}

func proposeGeneralSplineSections(ctx context.Context, a, b intersect.Patch, pair [2]int, o SplineShellOpenOptions) ([]SplineShellOpenSection, error) {
	trace, e := intersect.TraceSplinePair(ctx, a, b, o.Trace)
	if e != nil {
		return nil, e
	}
	if !trace.Complete || len(trace.Curves) != len(trace.BranchGraph.Branches) || len(trace.Curves) == 0 {
		return nil, geom.ErrConvergence
	}
	out := make([]SplineShellOpenSection, 0, len(trace.Curves))
	for i, branch := range trace.BranchGraph.Branches {
		curves := trace.Curves[i]
		if len(curves) == 0 || branch.StartUnresolved || branch.EndUnresolved {
			return nil, geom.ErrConvergence
		}
		s := SplineShellOpenSection{FaceA: pair[0], FaceB: pair[1], Branch: i, Closed: branch.Closed}
		if i == 0 {
			s.TubeCells = trace.Seeds
		}
		if !branch.Closed {
			for k, segment := range [2]intersect.TubeSegment{curves[0], curves[len(curves)-1]} {
				t, e := intersect.DiscoverRationalSplineTubeTerminal(ctx, segment, k, a, b, o.Endpoints)
				if e != nil && o.EnableBoundaryContactHints && isUnresolvedOpenSpline(e) {
					t, e = proposeTerminalFromBoundaryContact(ctx, segment, k, a, b, o)
				}
				if e != nil {
					return nil, e
				}
				endpoint, e := splineTerminalEndpointOnCharts(a, b, t.Operand, t.Proof.Parameters)
				if e != nil || endpoint.Edge.Side != t.Side {
					if e == nil {
						e = geom.ErrConvergence
					}
					return nil, e
				}
				if k == 0 {
					s.Start = endpoint
				} else {
					s.End = endpoint
				}
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func isUnresolvedOpenSpline(e error) bool {
	return errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrUnrepresentable) || errors.Is(e, intersect.ErrUnresolved)
}

func proposePlaneSplineSections(ctx context.Context, a, b intersect.Patch, pair [2]int,
	o intersect.SplinePairTraceOptions) ([]SplineShellOpenSection, error) {
	discovery := o.PlaneFallback
	discovery.Plane.Arcs.ResolveSubdivisionCorners = true
	r, e := intersect.DiscoverSplinePlanePairSections(ctx, a, b, o.Seeds.Patches, discovery)
	if e != nil {
		return nil, e
	}
	g, e := intersect.JoinSplineTubeBranches(ctx, a, b, r.Seeds, o.Branches)
	if e != nil {
		return nil, e
	}
	if g.Truncated || len(g.Branches) != r.OpenContours+r.ClosedContours || len(r.Terminals) != 2*r.OpenContours {
		return nil, geom.ErrConvergence
	}
	type terminalKey struct {
		attempt int
		upper   bool
	}
	ends := make(map[terminalKey]intersect.SplineOpenPlaneTerminal, len(r.Terminals))
	for _, t := range r.Terminals {
		key := terminalKey{t.Attempt, t.Upper}
		if _, ok := ends[key]; ok {
			return nil, geom.ErrConvergence
		}
		ends[key] = t
	}
	var out []SplineShellOpenSection
	closed := 0
	for i, br := range g.Branches {
		if br.StartUnresolved || br.EndUnresolved || len(br.Cells) == 0 {
			return nil, geom.ErrConvergence
		}
		s := SplineShellOpenSection{FaceA: pair[0], FaceB: pair[1], Branch: i, Closed: br.Closed}
		if i == 0 {
			s.TubeCells = r.Seeds
		}
		if br.Closed {
			closed++
			out = append(out, s)
			continue
		}
		var endpoints [2]SplineShellOpenEndpoint
		for end, cell := range [2]intersect.OrientedTubeCell{br.Cells[0], br.Cells[len(br.Cells)-1]} {
			key := terminalKey{cell.Cell, (end == 1) != cell.Reversed}
			t, ok := ends[key]
			if !ok {
				return nil, geom.ErrConvergence
			}
			delete(ends, key)
			endpoint, e := openPlaneTerminalEndpoint(a, b, t.Parameters)
			if e != nil {
				return nil, e
			}
			endpoints[end] = endpoint
		}
		s.Start, s.End = endpoints[0], endpoints[1]
		out = append(out, s)
	}
	if closed != r.ClosedContours || len(ends) != 0 {
		return nil, geom.ErrConvergence
	}
	return out, nil
}

func openPlaneTerminalEndpoint(a, b intersect.Patch, parameters [4]*big.Rat) (SplineShellOpenEndpoint, error) {
	return splineTerminalEndpointOnCharts(a, b, -1, parameters)
}

func splineTerminalEndpointOnCharts(a, b intersect.Patch, preferred int, parameters [4]*big.Rat) (SplineShellOpenEndpoint, error) {
	out := SplineShellOpenEndpoint{Operand: -1}
	var sides [2]*geom.SurfaceEdge
	for operand, patch := range [2]intersect.Patch{a, b} {
		lo, hi := [2]float64{patch.Min.X, patch.Min.Y}, [2]float64{patch.Max.X, patch.Max.Y}
		for axis := range 2 {
			v := parameters[2*operand+axis]
			l, h := v.Cmp(ratInset(lo[axis])), v.Cmp(ratInset(hi[axis]))
			if l < 0 || h > 0 {
				return out, geom.ErrConvergence
			}
			if l == 0 || h == 0 {
				if sides[operand] != nil {
					return out, geom.ErrConvergence
				}
				side := geom.SurfaceEdge(2 * axis)
				if h == 0 {
					side++
				}
				sides[operand] = &side
			}
		}
	}
	if preferred < 0 {
		preferred = 0
		if sides[0] == nil {
			preferred = 1
		}
	}
	if sides[preferred] == nil {
		return out, geom.ErrConvergence
	}
	return SplineShellOpenEndpoint{Operand: preferred, Edge: SplineTubeEndpoint{Side: *sides[preferred], Hint: parameters},
		OtherSide: sides[preferred^1]}, nil
}
