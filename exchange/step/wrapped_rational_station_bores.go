package step

import (
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedRationalStationBoreBody is a strict coaxial through-bore with the
// same normalized weighted profile on both sides and strictly separated scales
// throughout every axial slab. Each side has 1..16 original wall bands; their
// station grids may differ. The two annular caps share exactly equal end planes.
// Writer authorities are Outer/Inner.Walls, names and LengthUnit, not cached
// Solid or public source entity IDs. Numeric semantics are those of
// WrappedRationalStationLoftBody.
type WrappedRationalStationBoreBody struct {
	Name         string
	Outer, Inner WrappedRationalStationLoftBody
	CapNames     [2]string
	Solid        trimmed.Solid
	LengthUnit   float64
}

func WriteWrappedRationalStationBore(w io.Writer, m WrappedRationalStationBoreBody) error {
	side := func(b WrappedRationalStationLoftBody) WrappedStationLoftBody {
		return WrappedStationLoftBody{Walls: b.Walls}
	}
	return writeWrappedProfileBore(w, WrappedStationBoreBody{Name: m.Name, Outer: side(m.Outer), Inner: side(m.Inner), CapNames: m.CapNames, LengthUnit: m.LengthUnit}, false, true)
}

func ReadWrappedRationalStationBore(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedRationalStationBoreBody, error) {
	m, e := readWrappedProfileBore(r, l, o, false, true)
	if e != nil {
		return WrappedRationalStationBoreBody{}, e
	}
	side := func(b WrappedStationLoftBody) WrappedRationalStationLoftBody {
		return WrappedRationalStationLoftBody{Name: b.Name, Walls: b.Walls, CapNames: b.CapNames, Solid: b.Solid, LengthUnit: b.LengthUnit}
	}
	return WrappedRationalStationBoreBody{Name: m.Name, Outer: side(m.Outer), Inner: side(m.Inner), CapNames: m.CapNames, Solid: m.Solid, LengthUnit: m.LengthUnit}, nil
}

// WrappedRationalStationBoreFromSolid authenticates the actual annular cap
// trims, inward bore walls and original shared native rims before extracting
// the two closed profile sources. No cap or wall is inferred from a display.
func WrappedRationalStationBoreFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedRationalStationBoreBody, error) {
	if !validLengthUnit(unit) {
		return WrappedRationalStationBoreBody{}, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedRationalStationBore(ctx, o)
	if e != nil {
		return WrappedRationalStationBoreBody{}, e
	}
	if !p.Complete || len(p.OuterWalls) > 16 || len(p.InnerWalls) > 16 {
		return WrappedRationalStationBoreBody{}, geom.ErrConvergence
	}
	out := WrappedRationalStationBoreBody{Name: name, Solid: s, LengthUnit: unit}
	faces := s.Faces()
	for i, ids := range [][]int{p.OuterWalls, p.InnerWalls} {
		walls := make([]*geom.WrappedSurfacePatch, len(ids))
		for j, id := range ids {
			var ok bool
			walls[j], ok = faces[id].Surface.(*geom.WrappedSurfacePatch)
			if !ok {
				return WrappedRationalStationBoreBody{}, geom.ErrConvergence
			}
		}
		local, _, err := trimmed.NewWrappedRationalStationLoft(ctx, walls, 1e-10, o)
		if err != nil {
			return WrappedRationalStationBoreBody{}, err
		}
		b, err := WrappedRationalStationLoftFromSolid(ctx, local, "", unit, o)
		if err != nil {
			return WrappedRationalStationBoreBody{}, err
		}
		if i == 0 {
			out.Outer = b
		} else {
			out.Inner = b
		}
	}
	return out, nil
}
