package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateWrappedCircularStationLoft classifies a represented point exactly.
// Internal station disks are material interior except on their rim.
func (s Solid) LocateWrappedCircularStationLoft(ctx context.Context, point geom.Vec3, o geom.SurfacePatchOptions) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCircularStationLoft(ctx, o)
	if e != nil {
		return parametric.Outside, e
	}
	z := ratInset(point.Z)
	radius, ok := wrappedStationRadiusAt(p, z)
	if !ok {
		return parametric.Outside, nil
	}
	x, y := new(big.Rat).Sub(ratInset(point.X), p.Center[0]), new(big.Rat).Sub(ratInset(point.Y), p.Center[1])
	x.Add(x.Mul(x, x), y.Mul(y, y))
	c := x.Cmp(radius.Mul(radius, radius))
	if c > 0 {
		return parametric.Outside, nil
	}
	if c == 0 || z.Cmp(p.Heights[0]) == 0 || z.Cmp(p.Heights[len(p.Heights)-1]) == 0 {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}

func wrappedStationRadiusAt(p WrappedStationLoftProof, z *big.Rat) (*big.Rat, bool) {
	i := sort.Search(len(p.Heights), func(i int) bool { return z.Cmp(p.Heights[i]) <= 0 })
	if i == len(p.Heights) || i == 0 && z.Cmp(p.Heights[0]) < 0 {
		return nil, false
	}
	radius := new(big.Rat).Set(p.Radii[i])
	if i > 0 && z.Cmp(p.Heights[i]) < 0 {
		t := new(big.Rat).Quo(new(big.Rat).Sub(z, p.Heights[i-1]), new(big.Rat).Sub(p.Heights[i], p.Heights[i-1]))
		dr := new(big.Rat).Sub(radius, p.Radii[i-1])
		radius.Add(p.Radii[i-1], dr.Mul(dr, t))
	}
	return radius, true
}

// WrappedLoftLineHit records a point contact in the original line parameter
// with every incident wall/cap face identified at a proved station rim.
// Multiplicity is the maximum local polynomial root multiplicity.
type WrappedLoftLineHit struct {
	Parameter geom.ToroidLineRoot
	Faces     []int
	Parts     geom.CylinderBoundaryPart
}

// WrappedLoftLineOverlap represents a contact interval on a single original
// face. Adjacent intervals can share an endpoint and retain separate ancestry.
type WrappedLoftLineOverlap struct {
	Start, End geom.ToroidLineRoot
	Face       int
	Part       geom.CylinderBoundaryPart
}

type WrappedLoftLineIntersections struct {
	Hits     []WrappedLoftLineHit
	Overlaps []WrappedLoftLineOverlap
}

// IntersectWrappedCircularStationLoftLine proves all boundary contacts of a
// finite represented line with the exact piecewise conical loft. Internal
// station disk contacts are discarded; a rim reached by two bands is one
// contact with both original face IDs. A failed proof exposes no partial hits.
func (s Solid) IntersectWrappedCircularStationLoftLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64,
	o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	var result WrappedLoftLineIntersections
	if !origin.IsFinite() || !direction.IsFinite() || direction == (geom.Vec3{}) || !finite(minT) || !finite(maxT) ||
		minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return result, geom.ErrInvalid
	}
	p, bands, e := s.validateWrappedCircularStationLoft(ctx, o.Surface)
	if e != nil {
		return result, e
	}
	return intersectWrappedStationLoftBands(ctx, p, bands, origin, direction, minT, maxT, o)
}

func intersectWrappedStationLoftBands(ctx context.Context, p WrappedStationLoftProof, bands []*geom.WrappedSurfacePatch,
	origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	var result WrappedLoftLineIntersections
	exact := func(r geom.ToroidLineRoot) bool { return r.Min.Cmp(r.Max) == 0 }
	for i, w := range bands {
		r, e := geom.IntersectWrappedCircularFrustumLine(ctx, w, origin, direction, minT, maxT, o)
		if e != nil {
			return WrappedLoftLineIntersections{}, e
		}
		mask := geom.CylinderLateral
		if i == 0 {
			mask |= geom.CylinderLowerCap
		}
		if i == len(bands)-1 {
			mask |= geom.CylinderUpperCap
		}
		for _, h := range r.Hits {
			part := h.Parts & mask
			if part == 0 {
				continue
			}
			x := WrappedLoftLineHit{Parameter: h.Parameter, Parts: part}
			if part&geom.CylinderLateral != 0 {
				x.Faces = append(x.Faces, p.History.WallFaces[i])
			}
			for j, cap := range [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap} {
				if part&cap != 0 {
					x.Faces = append(x.Faces, p.History.CapFaces[j])
				}
			}
			result.Hits = append(result.Hits, x)
		}
		for _, h := range r.Overlaps {
			part := h.Part & mask
			if part == 0 {
				continue
			}
			face := p.History.WallFaces[i]
			if part == geom.CylinderLowerCap {
				face = p.History.CapFaces[0]
			} else if part == geom.CylinderUpperCap {
				face = p.History.CapFaces[1]
			}
			result.Overlaps = append(result.Overlaps, WrappedLoftLineOverlap{h.Start, h.End, face, part})
		}
	}
	sort.SliceStable(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
	hits := result.Hits[:0]
	// In a horizontal line at an internal station, neighboring supports use
	// precisely the same circle polynomial and the same original interval.
	// Thus corresponding isolating intervals identify the very same root.
	z := ratInset(origin.Z)
	horizontalStation := false
	if direction.Z == 0 {
		for _, h := range p.Heights[1 : len(p.Heights)-1] {
			horizontalStation = horizontalStation || z.Cmp(h) == 0
		}
	}
	for _, h := range result.Hits {
		if len(hits) != 0 {
			last := &hits[len(hits)-1]
			a, b := last.Parameter, h.Parameter
			if a.Max.Cmp(b.Min) >= 0 {
				sameInterval := a.Min.Cmp(b.Min) == 0 && a.Max.Cmp(b.Max) == 0
				if sameInterval && (horizontalStation || exact(a) && exact(b)) {
					for _, f := range h.Faces {
						last.Faces = appendUniqueFace(last.Faces, f)
					}
					last.Parts |= h.Parts
					last.Parameter.Multiplicity = max(a.Multiplicity, b.Multiplicity)
					continue
				}
				return WrappedLoftLineIntersections{}, geom.ErrConvergence
			}
		}
		hits = append(hits, h)
	}
	for i := range hits {
		sort.Ints(hits[i].Faces)
	}
	sort.SliceStable(result.Overlaps, func(i, j int) bool {
		a, b := result.Overlaps[i], result.Overlaps[j]
		if c := a.Start.Min.Cmp(b.Start.Min); c != 0 {
			return c < 0
		}
		return a.Face < b.Face
	})
	result.Hits = hits
	return result, nil
}

func appendUniqueFace(ids []int, id int) []int {
	for _, old := range ids {
		if old == id {
			return ids
		}
	}
	return append(ids, id)
}
