package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedRationalStationLineOptions bounds a strict isolated-contact query.
// Geometry keeps its existing per-stage/per-quadrant budgets, at most16 bands.
// MaxEvents bounds candidate events (before merging), default128, maximum256. MaxPointOperations (default65536, maximum1M)
// is divided across at most MaxEvents+1 exact gap classifications. Each gap
// independently authenticates the source under Geometry.Surface; that bounded
// source quota is separate from its point arithmetic quota. Ledger operations
// are structurally bounded by256 events and rational coefficients by65536 bits.
type WrappedRationalStationLineOptions struct {
	Geometry                      geom.ToroidLineOptions
	MaxEvents, MaxPointOperations int
}

// WrappedRationalLineInterior is an open parameter interval in material interior.
// Ends retain certified algebraic root brackets or exact rational clip endpoints;
// they are not rounded parameter values. Isolated tangent points appear only in
// Hits, never as positive-length material. Clip endpoints need not be boundary.
type WrappedRationalLineInterior struct{ Start, End geom.ToroidLineRoot }

type WrappedRationalStationLine struct {
	Hits     []WrappedLoftLineHit
	Interior []WrappedRationalLineInterior
}

// ClipWrappedRationalStationLoftLineRational reports all retained isolated
// contacts and interior intervals of a rational line with minT < maxT. Noncircular conic
// quadrants are solved on original supports. Internal band caps are discarded;
// exact crease contacts retain both original wall owners. Coincident boundary
// intervals, touching non-exact isolation brackets and unresolved endpoint gaps
// fail atomically rather than guessing equality or a material interval.
func (s Solid) ClipWrappedRationalStationLoftLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o WrappedRationalStationLineOptions) (WrappedRationalStationLine, error) {
	empty := WrappedRationalStationLine{}
	if o.MaxEvents == 0 {
		o.MaxEvents = 128
	}
	if o.MaxPointOperations == 0 {
		o.MaxPointOperations = 65536
	}
	if ctx == nil || o.MaxEvents < 1 || o.MaxEvents > 256 || o.MaxPointOperations < 1 || o.MaxPointOperations > 1000000 || !finite(o.Geometry.ParameterTolerance) || o.Geometry.ParameterTolerance <= 0 || o.Geometry.MaxSubdivisions < 0 || o.Geometry.MaxSubdivisions > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	bits := o.Geometry.Surface.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if bits < 64 || bits > 65536 {
		return empty, geom.ErrInvalid
	}
	valid := func(q *big.Rat) bool { return q != nil && q.Num().BitLen() <= bits && q.Denom().BitLen() <= bits }
	if !valid(minT) || !valid(maxT) || minT.Cmp(maxT) >= 0 {
		return empty, geom.ErrInvalid
	}
	nonzero := false
	for i := 0; i < 3; i++ {
		if !valid(origin[i]) || !valid(direction[i]) {
			return empty, geom.ErrInvalid
		}
		nonzero = nonzero || direction[i].Sign() != 0
	}
	if !nonzero {
		return empty, geom.ErrInvalid
	}
	p, bands, e := s.validateWrappedProfileStationLoft(ctx, o.Geometry.Surface, false)
	if e != nil {
		return empty, e
	}
	type selected struct{ band, station int }
	var selectedBands []selected
	if direction[2].Sign() == 0 {
		band, station := -1, -1
		for i, h := range p.Heights {
			c := origin[2].Cmp(h)
			if c == 0 {
				station = i
				band = i
				if band == len(bands) {
					band--
				}
				break
			}
			if c < 0 {
				band = i - 1
				break
			}
		}
		if band < 0 {
			return empty, nil
		}
		selectedBands = append(selectedBands, selected{band, station})
	} else {
		for i := range bands {
			selectedBands = append(selectedBands, selected{i, -1})
		}
	}
	cloneRoot := func(q geom.ToroidLineRoot) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(q.Min), Max: new(big.Rat).Set(q.Max), Multiplicity: q.Multiplicity}
	}
	var hits []WrappedLoftLineHit
	for _, selection := range selectedBands {
		band := selection.band
		report, e := geom.IntersectWrappedFrustumLineRational(ctx, bands[band], origin, direction, minT, maxT, o.Geometry)
		if e != nil {
			return empty, e
		}
		mask := geom.CylinderLateral
		if band == 0 {
			mask |= geom.CylinderLowerCap
		}
		if band == len(bands)-1 {
			mask |= geom.CylinderUpperCap
		}
		for _, span := range report.Overlaps {
			if span.Part&mask != 0 {
				return empty, geom.ErrConvergence
			}
		}
		for _, hit := range report.Hits {
			parts := hit.Parts & mask
			if parts == 0 {
				continue
			}
			q := hit.Parameter
			if !valid(q.Min) || !valid(q.Max) || q.Min.Cmp(q.Max) > 0 || q.Min.Cmp(minT) < 0 || q.Max.Cmp(maxT) > 0 {
				return empty, geom.ErrConvergence
			}
			event := WrappedLoftLineHit{Parameter: cloneRoot(q), Parts: parts}
			if parts&geom.CylinderLateral != 0 {
				event.Faces = append(event.Faces, p.History.WallFaces[band])
				station := selection.station
				if station > 0 && station < len(p.Heights)-1 {
					event.Faces = appendUniqueFace(event.Faces, p.History.WallFaces[station-1])
					event.Faces = appendUniqueFace(event.Faces, p.History.WallFaces[station])
				}
			}
			if parts&geom.CylinderLowerCap != 0 {
				event.Faces = appendUniqueFace(event.Faces, p.History.CapFaces[0])
			}
			if parts&geom.CylinderUpperCap != 0 {
				event.Faces = appendUniqueFace(event.Faces, p.History.CapFaces[1])
			}
			if len(hits) >= o.MaxEvents {
				return empty, geom.ErrConvergence
			}
			hits = append(hits, event)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Parameter.Min.Cmp(hits[j].Parameter.Min) < 0 })
	merged := make([]WrappedLoftLineHit, 0, len(hits))
	for _, hit := range hits {
		if len(merged) > 0 {
			previous := &merged[len(merged)-1]
			a, b := previous.Parameter, hit.Parameter
			if a.Max.Cmp(b.Min) >= 0 {
				if a.Min.Cmp(a.Max) != 0 || b.Min.Cmp(b.Max) != 0 || a.Min.Cmp(b.Min) != 0 {
					return empty, geom.ErrConvergence
				}
				for _, face := range hit.Faces {
					previous.Faces = appendUniqueFace(previous.Faces, face)
				}
				previous.Parts |= hit.Parts
				previous.Parameter.Multiplicity = max(a.Multiplicity, b.Multiplicity)
				continue
			}
		}
		merged = append(merged, hit)
	}
	for i := range merged {
		sort.Ints(merged[i].Faces)
	}
	exact := func(q *big.Rat) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(q), Max: new(big.Rat).Set(q)}
	}
	ends := []geom.ToroidLineRoot{exact(minT)}
	for _, hit := range merged {
		ends = append(ends, cloneRoot(hit.Parameter))
	}
	ends = append(ends, exact(maxT))
	allowance := o.MaxPointOperations / (len(ends) - 1)
	if allowance < 1 {
		return empty, geom.ErrConvergence
	}
	result := WrappedRationalStationLine{Hits: merged}
	for i := 1; i < len(ends); i++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		left, right := ends[i-1], ends[i]
		cmp := left.Max.Cmp(right.Min)
		if cmp >= 0 {
			if cmp == 0 && left.Min.Cmp(left.Max) == 0 && right.Min.Cmp(right.Max) == 0 {
				continue
			}
			return empty, geom.ErrConvergence
		}
		t := new(big.Rat).Quo(new(big.Rat).Add(left.Max, right.Min), big.NewRat(2, 1))
		if !valid(t) {
			return empty, geom.ErrConvergence
		}
		var point [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			v := new(big.Rat).Mul(t, direction[axis])
			if !valid(v) {
				return empty, geom.ErrConvergence
			}
			point[axis] = new(big.Rat).Add(origin[axis], v)
			if !valid(point[axis]) {
				return empty, geom.ErrConvergence
			}
		}
		classification, e := s.LocateWrappedRationalStationLoftRational(ctx, point, WrappedRationalStationPointOptions{Source: o.Geometry.Surface, MaxPointOperations: allowance, MaxCoefficientBits: bits})
		if e != nil {
			return empty, e
		}
		switch classification.Location {
		case parametric.Inside:
			result.Interior = append(result.Interior, WrappedRationalLineInterior{cloneRoot(left), cloneRoot(right)})
		case parametric.Outside:
		case parametric.Boundary:
			return empty, geom.ErrConvergence
		default:
			return empty, geom.ErrConvergence
		}
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
