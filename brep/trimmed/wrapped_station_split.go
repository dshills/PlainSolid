package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedStationLoftSplitHistory relates each original band to the retained
// wall face in the lower and upper results. -1 means no retained portion.
type WrappedStationLoftSplitHistory struct {
	LowerWall, UpperWall []int
	OriginalWall         []int
	LowerCut, UpperCut   int
	ExistingStation      bool
}

// SplitWrappedCircularStationLoftAtZ cuts a certified circular loft by an
// interior horizontal plane. Original unsplit charts are reused. An
// intersected frustum is restricted without changing its U basis; both
// results get exact caps and an independently valid closed shell.
func (s Solid) SplitWrappedCircularStationLoftAtZ(ctx context.Context, z float64,
	o geom.SurfacePatchOptions) (Solid, Solid, WrappedStationLoftSplitHistory, error) {
	bad := WrappedStationLoftSplitHistory{LowerCut: -1, UpperCut: -1}
	if ctx == nil || !finite(z) {
		return Solid{}, Solid{}, bad, geom.ErrInvalid
	}
	p, bands, e := s.validateWrappedCircularStationLoft(ctx, o)
	if e != nil {
		return Solid{}, Solid{}, bad, e
	}
	lo, hi, history, e := splitWrappedStationBands(ctx, p, bands, z, o)
	if e != nil {
		return Solid{}, Solid{}, bad, e
	}
	lower, a, e := NewWrappedCircularStationLoft(ctx, lo, s.tolerance, o)
	if e != nil {
		return Solid{}, Solid{}, bad, e
	}
	upper, b, e := NewWrappedCircularStationLoft(ctx, hi, s.tolerance, o)
	if e != nil {
		return Solid{}, Solid{}, bad, e
	}
	history.LowerCut, history.UpperCut = a.CapFaces[1], b.CapFaces[0]
	return lower, upper, history, nil
}

func splitWrappedStationBands(ctx context.Context, p WrappedStationLoftProof, bands []*geom.WrappedSurfacePatch,
	z float64, o geom.SurfacePatchOptions) ([]*geom.WrappedSurfacePatch, []*geom.WrappedSurfacePatch, WrappedStationLoftSplitHistory, error) {
	bad := WrappedStationLoftSplitHistory{LowerCut: -1, UpperCut: -1}
	q := ratInset(z)
	if q.Cmp(p.Heights[0]) <= 0 || q.Cmp(p.Heights[len(p.Heights)-1]) >= 0 {
		return nil, nil, bad, geom.ErrDomain
	}
	h := WrappedStationLoftSplitHistory{
		LowerWall: make([]int, len(bands)), UpperWall: make([]int, len(bands)), OriginalWall: append([]int(nil), p.History.WallFaces...),
		LowerCut: -1, UpperCut: -1,
	}
	var lower, upper []*geom.WrappedSurfacePatch
	for i, w := range bands {
		if e := ctx.Err(); e != nil {
			return nil, nil, bad, e
		}
		h.LowerWall[i], h.UpperWall[i] = -1, -1
		switch {
		case q.Cmp(p.Heights[i+1]) >= 0:
			h.LowerWall[i] = len(lower)
			lower = append(lower, w)
			h.ExistingStation = h.ExistingStation || q.Cmp(p.Heights[i+1]) == 0
		case q.Cmp(p.Heights[i]) <= 0:
			h.UpperWall[i] = len(upper)
			upper = append(upper, w)
		default:
			a, b, e := geom.SplitWrappedFrustumAtZ(ctx, w, z, o)
			if e != nil {
				return nil, nil, bad, e
			}
			h.LowerWall[i], h.UpperWall[i] = len(lower), len(upper)
			lower, upper = append(lower, a), append(upper, b)
		}
	}
	return lower, upper, h, nil
}

// WrappedStationBoreSplitHistory records both wall families independently.
// LowerCut and UpperCut identify the same new annular section in each shell.
type WrappedStationBoreSplitHistory struct {
	Outer, Inner       WrappedStationLoftSplitHistory
	LowerCut, UpperCut int
}

// SplitWrappedCircularStationBoreAtZ cuts both original wall families and
// closes each result with its exact annular section. Outer and inner station
// partitions may differ; each is restricted independently.
func (s Solid) SplitWrappedCircularStationBoreAtZ(ctx context.Context, z float64,
	o geom.SurfacePatchOptions) (Solid, Solid, WrappedStationBoreSplitHistory, error) {
	bad := WrappedStationBoreSplitHistory{LowerCut: -1, UpperCut: -1}
	if ctx == nil || !finite(z) {
		return Solid{}, Solid{}, bad, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCircularStationBore(ctx, o)
	if e != nil {
		return Solid{}, Solid{}, bad, e
	}
	var capped [2][2]Solid // wall family, lower or upper piece
	var history [2]WrappedStationLoftSplitHistory
	for i, q := range [2]WrappedStationLoftProof{p.Outer, p.Inner} {
		ids := [2][]int{p.OuterWalls, p.InnerWalls}[i]
		bands := make([]*geom.WrappedSurfacePatch, len(ids))
		for j, id := range ids {
			bands[j] = s.faces[id].Surface.(*geom.WrappedSurfacePatch)
		}
		lo, hi, h, e := splitWrappedStationBands(ctx, q, bands, z, o)
		if e != nil {
			return Solid{}, Solid{}, bad, e
		}
		h.OriginalWall = append([]int(nil), ids...)
		history[i] = h
		for k, w := range [][]*geom.WrappedSurfacePatch{lo, hi} {
			r, _, e := NewWrappedCircularStationLoft(ctx, w, s.tolerance, o)
			if e != nil {
				return Solid{}, Solid{}, bad, e
			}
			capped[i][k] = r
		}
	}
	var out [2]Solid
	var caps [2]int
	for k := range out {
		r, h, e := CombineCoaxialWrappedStationLofts(ctx, capped[0][k], capped[1][k], parametric.Difference, o)
		if e != nil {
			return Solid{}, Solid{}, bad, e
		}
		p, e := r.ValidateWrappedCircularStationBore(ctx, o)
		if e != nil {
			return Solid{}, Solid{}, bad, e
		}
		caps[k] = p.Caps[1-k]
		out[k] = r
		for i, bands := range [][]WrappedStationBooleanBand{h.Bands, h.InnerBands} {
			hist := &history[i]
			m := hist.LowerWall
			if k == 1 {
				m = hist.UpperWall
			}
			for j, face := range m {
				if face >= 0 {
					if face >= len(bands) {
						return Solid{}, Solid{}, bad, geom.ErrConvergence
					}
					m[j] = bands[face].ResultWall
				}
			}
		}
	}
	for i := range history {
		history[i].LowerCut, history[i].UpperCut = caps[0], caps[1]
	}
	return out[0], out[1], WrappedStationBoreSplitHistory{history[0], history[1], caps[0], caps[1]}, nil
}
