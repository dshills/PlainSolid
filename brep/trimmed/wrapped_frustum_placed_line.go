//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// IntersectPlacedWrappedFrustumLineRational intersects a complete original
// capped frustum with an exact world occurrence line. Independently
// authored homothetic, noncircular rational profiles are supported. Every
// hit and overlap retains its original native face rather than a world copy.
func (s Solid) IntersectPlacedWrappedFrustumLineRational(ctx context.Context, pose placement.RationalRigid,
	origin, direction [3]*big.Rat, minT, maxT *big.Rat, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	var out WrappedLoftLineIntersections
	p, e := pose.TransformPoint(ctx, origin, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return out, e
	}
	d, e := pose.TransformVector(ctx, direction, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return out, e
	}
	w, ids, e := s.provePlacedWeightedFrustum(ctx, o.Surface)
	if e != nil {
		return out, e
	}
	r, e := geom.IntersectWrappedFrustumLineRational(ctx, w, p, d, minT, maxT, o)
	if e != nil {
		return out, e
	}
	return placedFrustumLineOwners(r, ids)
}

func placedFrustumLineOwners(r geom.CylinderLineIntersections, ids [3]int) (out WrappedLoftLineIntersections, err error) {
	for _, h := range r.Hits {
		hit := WrappedLoftLineHit{Parameter: h.Parameter, Parts: h.Parts}
		for i, part := range placedFrustumParts {
			if h.Parts&part != 0 {
				hit.Faces = append(hit.Faces, ids[i])
			}
		}
		sort.Ints(hit.Faces)
		out.Hits = append(out.Hits, hit)
	}
	for _, h := range r.Overlaps {
		face := -1
		for i, part := range placedFrustumParts {
			if h.Part == part {
				face = ids[i]
				break
			}
		}
		if face < 0 {
			return WrappedLoftLineIntersections{}, geom.ErrConvergence
		}
		out.Overlaps = append(out.Overlaps, WrappedLoftLineOverlap{Start: h.Start, End: h.End, Face: face, Part: h.Part})
	}
	return out, nil
}

var placedFrustumParts = [3]geom.CylinderBoundaryPart{geom.CylinderLateral, geom.CylinderLowerCap, geom.CylinderUpperCap}

func (s Solid) provePlacedWeightedFrustum(ctx context.Context, o geom.SurfacePatchOptions) (*geom.WrappedSurfacePatch, [3]int, error) {
	w, proof, e := s.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return nil, [3]int{}, e
	}
	ids := [3]int{-1, -1, -1}
	for i, f := range s.faces {
		switch x := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			if x != w || ids[0] >= 0 {
				return nil, [3]int{}, geom.ErrConvergence
			}
			ids[0] = i
		case geom.PlaneSurface:
			z := ratInset(x.Frame().Origin().Z)
			switch {
			case z.Cmp(proof.LowerZ) == 0 && ids[1] < 0:
				ids[1] = i
			case z.Cmp(proof.UpperZ) == 0 && ids[2] < 0:
				ids[2] = i
			default:
				return nil, [3]int{}, geom.ErrConvergence
			}
		default:
			return nil, [3]int{}, geom.ErrConvergence
		}
	}
	for _, id := range ids {
		if id < 0 {
			return nil, [3]int{}, geom.ErrConvergence
		}
	}
	return w, ids, nil
}

// LocatePlacedWrappedFrustumRational classifies the complete original capped
// profile after an exact proper world occurrence placement.
func (s Solid) LocatePlacedWrappedFrustumRational(ctx context.Context, pose placement.RationalRigid, world [3]*big.Rat, o geom.SurfacePatchOptions) (parametric.Location, error) {
	p, e := pose.TransformPoint(ctx, world, true, o.MaxCoefficientBits)
	if e != nil {
		return parametric.Outside, e
	}
	w, _, e := s.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return parametric.Outside, e
	}
	x, e := geom.LocateWrappedFrustumRational(ctx, w, p, o)
	if e != nil {
		return parametric.Outside, e
	}
	switch x {
	case geom.CylinderInside:
		return parametric.Inside, nil
	case geom.CylinderBoundary:
		return parametric.Boundary, nil
	}
	return parametric.Outside, nil
}

// PlacedWeightedFrustumDistance retains an exact world boundary witness and
// original native owner for the complete weighted-frustum distance query.
type PlacedWeightedFrustumDistance struct {
	geom.CylinderDistanceBounds
	Face int
}

// BoundPointPlacedWrappedFrustumDistanceRational proves the global nearest
// boundary distance in the original support basis and maps the certified
// rational witness back to world coordinates. Noncircular profiles qualify.
func (s Solid) BoundPointPlacedWrappedFrustumDistanceRational(ctx context.Context, pose placement.RationalRigid, world [3]*big.Rat, o geom.SurfaceDistanceOptions) (PlacedWeightedFrustumDistance, error) {
	var out PlacedWeightedFrustumDistance
	p, e := pose.TransformPoint(ctx, world, true, o.MaxCoefficientBits)
	if e != nil {
		return out, e
	}
	w, ids, e := s.provePlacedWeightedFrustum(ctx, geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return out, e
	}
	r, e := geom.BoundPointWrappedFrustumDistanceRational(ctx, w, p, o)
	if e != nil {
		return out, e
	}
	idx := -1
	for i, k := range placedFrustumParts {
		if k == r.Part {
			idx = i
			break
		}
	}
	if idx < 0 {
		return out, geom.ErrConvergence
	}
	r.Witness, e = pose.TransformPoint(ctx, r.Witness, false, o.MaxCoefficientBits)
	if e != nil {
		return out, e
	}
	return PlacedWeightedFrustumDistance{CylinderDistanceBounds: r, Face: ids[idx]}, nil
}

// PlacedFrustumMaterialLine preserves native boundary ownership and the
// original finite parameter intervals occupied by closed material. Repeated
// isolated contacts may contribute a singleton span. Endpoints inside the
// material remain distinct from original boundary events.
type PlacedFrustumMaterialLine struct {
	Boundary WrappedLoftLineIntersections
	Spans    [][2]geom.ToroidLineRoot
}

// ClipPlacedWrappedFrustumLineRational classifies root-free cells from the
// complete original frustum boundary proof. Generator and cap overlaps
// remain explicit, and algebraic endpoints keep their original brackets.
func (s Solid) ClipPlacedWrappedFrustumLineRational(ctx context.Context, pose placement.RationalRigid, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions) (out PlacedFrustumMaterialLine, err error) {
	defer func() {
		if err != nil {
			out = PlacedFrustumMaterialLine{}
		}
	}()
	p, e := pose.TransformPoint(ctx, origin, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return out, e
	}
	d, e := pose.TransformVector(ctx, direction, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return out, e
	}
	w, ids, e := s.provePlacedWeightedFrustum(ctx, o.Surface)
	if e != nil {
		return out, e
	}
	roots, e := geom.IntersectWrappedFrustumLineRational(ctx, w, p, d, minT, maxT, o)
	if e != nil {
		return out, e
	}
	out.Boundary, e = placedFrustumLineOwners(roots, ids)
	if e != nil {
		return out, e
	}
	type lineMark struct {
		t        geom.ToroidLineRoot
		boundary bool
	}
	point := func(t *big.Rat) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(t), Max: new(big.Rat).Set(t)}
	}
	marks := []lineMark{{point(minT), false}, {point(maxT), false}}
	for _, h := range roots.Hits {
		marks = append(marks, lineMark{h.Parameter, true})
	}
	for _, o := range roots.Overlaps {
		marks = append(marks, lineMark{o.Start, true}, lineMark{o.End, true})
	}
	sort.Slice(marks, func(i, j int) bool { return marks[i].t.Min.Cmp(marks[j].t.Min) < 0 })
	unique := marks[:0]
	for _, m := range marks {
		if len(unique) > 0 {
			u := &unique[len(unique)-1]
			if m.t.Min.Cmp(u.t.Min) == 0 && m.t.Max.Cmp(u.t.Max) == 0 {
				u.boundary = u.boundary || m.boundary
				u.t.Multiplicity = max(u.t.Multiplicity, m.t.Multiplicity)
				continue
			}
			if m.t.Min.Cmp(u.t.Max) <= 0 {
				return out, geom.ErrConvergence
			}
		}
		unique = append(unique, m)
	}
	bits := o.Surface.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	eval := func(t *big.Rat) (geom.CylinderLocation, error) {
		if e := ctx.Err(); e != nil {
			return geom.CylinderUnknown, e
		}
		var xyz [3]*big.Rat
		for i := range xyz {
			v := new(big.Rat).Mul(t, d[i])
			if v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
				return geom.CylinderUnknown, geom.ErrConvergence
			}
			xyz[i] = v.Add(v, p[i])
		}
		return geom.LocateWrappedFrustumRational(ctx, w, xyz, o.Surface)
	}
	inside := make([]bool, len(unique)+1)
	for i := 1; i < len(unique); i++ {
		lo, hi := unique[i-1].t.Max, unique[i].t.Min
		x, e := eval(new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1)))
		if e != nil {
			return out, e
		}
		if x == geom.CylinderBoundary {
			known := false
			for _, p := range roots.Overlaps {
				if lo.Cmp(p.Start.Max) >= 0 && hi.Cmp(p.End.Min) <= 0 {
					known = true
					break
				}
			}
			if !known {
				return out, geom.ErrConvergence
			}
		}
		inside[i] = x == geom.CylinderInside || x == geom.CylinderBoundary
	}
	for i, m := range unique {
		if inside[i] {
			if len(out.Spans) == 0 {
				return out, geom.ErrConvergence
			}
			out.Spans[len(out.Spans)-1][1] = m.t
			continue
		}
		if m.boundary || inside[i+1] {
			out.Spans = append(out.Spans, [2]geom.ToroidLineRoot{m.t, m.t})
		}
	}
	if len(unique) == 1 && !unique[0].boundary {
		x, e := eval(minT)
		if e != nil {
			return out, e
		}
		if x == geom.CylinderInside || x == geom.CylinderBoundary {
			t := unique[0].t
			out.Spans = append(out.Spans, [2]geom.ToroidLineRoot{t, t})
		}
	}
	return out, ctx.Err()
}
