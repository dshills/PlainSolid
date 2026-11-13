package offset

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedRoundedCylinderHistory records the original faces incident to each
// result face. A generated outward blend at an original sharp rim has both
// the original cap and lateral face as its sources.
type WrappedRoundedCylinderHistory struct {
	ResultLateral int
	ResultCaps    [2]int
	ResultBlends  [2]int // -1 for a retained sharp rim
	SourceFaces   [][]int
}

// WrappedRoundedCylinder offsets the complete canonical circular cylinder or
// its exact rim-filleted variant in the Euclidean metric. The result is the
// boundary of dilation by a ball for positive distance, and erosion for
// negative distance. The cap planes and lateral wall move by the same signed
// distance; existing rolling-circle centers stay fixed. Positive distance
// rounds a sharp rim; negative distance retains an originally sharp rim.
// Offset distances that eliminate an existing blend, the cylindrical
// middle, or the positive-radius canonical toroidal family are rejected.
func WrappedRoundedCylinder(ctx context.Context, solid trimmed.Solid, distance float64,
	o geom.SurfacePatchOptions) (trimmed.Solid, WrappedRoundedCylinderHistory, error) {
	empty := WrappedRoundedCylinderHistory{ResultLateral: -1, ResultCaps: [2]int{-1, -1}, ResultBlends: [2]int{-1, -1}}
	if ctx == nil {
		return trimmed.Solid{}, empty, geom.ErrInvalid
	}
	d := new(big.Rat).SetFloat64(distance)
	if d == nil {
		return trimmed.Solid{}, empty, geom.ErrInvalid
	}
	p, e := solid.ValidateWrappedCylinderRimFillet(ctx, o)
	var w *geom.WrappedSurfacePatch
	if e == nil {
		w = solid.Faces()[p.Lateral].Surface.(*geom.WrappedSurfacePatch)
	} else {
		if !errors.Is(e, geom.ErrConvergence) || solid.Topology().Faces != 3 {
			return trimmed.Solid{}, empty, e
		}
		var q geom.WrappedCylinderEmbedding
		w, q, e = solid.ValidateCanonicalWrappedCylinder(ctx, o)
		if e != nil {
			return trimmed.Solid{}, empty, e
		}
		if !q.Circular {
			return trimmed.Solid{}, empty, geom.ErrConvergence
		}
		p = trimmed.WrappedCylinderFilletProof{
			Radius: q.Radius, LowerZ: q.LowerZ, UpperZ: q.UpperZ,
			Fillets: [2]*big.Rat{new(big.Rat), new(big.Rat)},
			Lateral: -1, Caps: [2]int{-1, -1}, Blends: [2]int{-1, -1},
		}
		for i, face := range solid.Faces() {
			switch chart := face.Surface.(type) {
			case *geom.WrappedSurfacePatch:
				if chart == w {
					p.Lateral = i
				}
			case geom.PlaneSurface:
				z := new(big.Rat).SetFloat64(chart.Frame().Origin().Z)
				if z.Cmp(q.LowerZ) == 0 {
					p.Caps[0] = i
				} else if z.Cmp(q.UpperZ) == 0 {
					p.Caps[1] = i
				}
			}
		}
		if p.Lateral < 0 || p.Caps[0] < 0 || p.Caps[1] < 0 {
			return trimmed.Solid{}, empty, geom.ErrConvergence
		}
	}
	if d.Sign() == 0 {
		h := WrappedRoundedCylinderHistory{p.Lateral, p.Caps, p.Blends, make([][]int, solid.Topology().Faces)}
		for i := range h.SourceFaces {
			h.SourceFaces[i] = []int{i}
		}
		return solid, h, nil
	}
	lower, upper := new(big.Rat).Sub(p.LowerZ, d), new(big.Rat).Add(p.UpperZ, d)
	if lower.Cmp(upper) >= 0 || new(big.Rat).Add(p.Radius, d).Sign() <= 0 {
		return trimmed.Solid{}, empty, geom.ErrDomain
	}
	var radii [2]float64
	for i, r := range p.Fillets {
		if r.Sign() == 0 && d.Sign() < 0 {
			continue
		}
		q := new(big.Rat).Add(r, d)
		if q.Sign() <= 0 {
			return trimmed.Solid{}, empty, geom.ErrDomain
		}
		radii[i], e = geom.RepresentedExactScalar(q)
		if e != nil {
			return trimmed.Solid{}, empty, e
		}
	}
	w, _, e = geom.DraftWrappedFrustumExact(ctx, w, d, d, o)
	if e != nil {
		return trimmed.Solid{}, empty, e
	}
	w, e = geom.ReheightWrappedCylinder(ctx, w, lower, upper, o)
	if e != nil {
		return trimmed.Solid{}, empty, e
	}
	out, _, e := trimmed.NewWrappedCylinder(ctx, w, solid.Tolerance(), o)
	if e != nil {
		return trimmed.Solid{}, empty, e
	}
	h := WrappedRoundedCylinderHistory{0, [2]int{1, 2}, [2]int{-1, -1}, nil}
	if radii != [2]float64{} {
		var b trimmed.WrappedCylinderRimFilletHistory
		out, b, e = trimmed.FilletWrappedCylinderRims(ctx, out, radii[0], radii[1], o)
		if e != nil {
			return trimmed.Solid{}, empty, e
		}
		h.ResultLateral, h.ResultCaps, h.ResultBlends = b.Lateral, b.Caps, b.Blends
	}
	h.SourceFaces = make([][]int, out.Topology().Faces)
	h.SourceFaces[h.ResultLateral] = []int{p.Lateral}
	for i, id := range h.ResultCaps {
		h.SourceFaces[id] = []int{p.Caps[i]}
		if blend := h.ResultBlends[i]; blend >= 0 {
			if p.Blends[i] >= 0 {
				h.SourceFaces[blend] = []int{p.Blends[i]}
			} else {
				h.SourceFaces[blend] = []int{p.Lateral, p.Caps[i]}
			}
		}
	}
	return out, h, nil
}
