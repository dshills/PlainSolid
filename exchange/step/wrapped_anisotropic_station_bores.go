package step

import (
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedAnisotropicStationBoreBody is a strict coaxial through-bore with the
// same normalized weighted profile on both sides and strictly separated X/Y scales
// throughout every axial slab. Each side has 1..16 original wall bands; their
// station grids may differ. The two annular caps share exactly equal end planes.
// The original inner and outer station grids and aspect ratios may differ.
// Writer authorities are Outer/Inner.Walls, names and LengthUnit, not cached
// Solid or public source entity IDs. Numeric semantics are those of
// WrappedAnisotropicStationLoftBody.
type WrappedAnisotropicStationBoreBody struct {
	Name         string
	Outer, Inner WrappedAnisotropicStationLoftBody
	CapNames     [2]string
	Solid        trimmed.Solid
	LengthUnit   float64
}

func WriteWrappedAnisotropicStationBore(w io.Writer, m WrappedAnisotropicStationBoreBody) error {
	side := func(b WrappedAnisotropicStationLoftBody) WrappedStationLoftBody {
		return WrappedStationLoftBody{Walls: b.Walls}
	}
	return writeWrappedProfileBore(w, WrappedStationBoreBody{Name: m.Name, Outer: side(m.Outer), Inner: side(m.Inner), CapNames: m.CapNames, LengthUnit: m.LengthUnit}, false, true, true)
}

func ReadWrappedAnisotropicStationBore(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedAnisotropicStationBoreBody, error) {
	m, e := readWrappedProfileBore(r, l, o, false, true, true)
	if e != nil {
		return WrappedAnisotropicStationBoreBody{}, e
	}
	side := func(b WrappedStationLoftBody) WrappedAnisotropicStationLoftBody {
		return WrappedAnisotropicStationLoftBody{Name: b.Name, Walls: b.Walls, CapNames: b.CapNames, Solid: b.Solid, LengthUnit: b.LengthUnit}
	}
	return WrappedAnisotropicStationBoreBody{Name: m.Name, Outer: side(m.Outer), Inner: side(m.Inner), CapNames: m.CapNames, Solid: m.Solid, LengthUnit: m.LengthUnit}, nil
}

// WrappedAnisotropicStationBoreFromSolid authenticates the actual annular cap
// trims, inward bore walls and original shared native rims before extracting
// the two closed profile sources. No cap or wall is inferred from a display.
func WrappedAnisotropicStationBoreFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedAnisotropicStationBoreBody, error) {
	if !validLengthUnit(unit) {
		return WrappedAnisotropicStationBoreBody{}, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedAnisotropicStationBore(ctx, o)
	if e != nil {
		return WrappedAnisotropicStationBoreBody{}, e
	}
	if !p.Complete || len(p.OuterWalls) > 16 || len(p.InnerWalls) > 16 {
		return WrappedAnisotropicStationBoreBody{}, geom.ErrConvergence
	}
	out := WrappedAnisotropicStationBoreBody{Name: name, Solid: s, LengthUnit: unit}
	faces := s.Faces()
	for i, ids := range [][]int{p.OuterWalls, p.InnerWalls} {
		walls := make([]*geom.WrappedSurfacePatch, len(ids))
		for j, id := range ids {
			var ok bool
			walls[j], ok = faces[id].Surface.(*geom.WrappedSurfacePatch)
			if !ok {
				return WrappedAnisotropicStationBoreBody{}, geom.ErrConvergence
			}
		}
		local, _, err := trimmed.NewWrappedAnisotropicStationLoft(ctx, walls, 1e-10, o)
		if err != nil {
			return WrappedAnisotropicStationBoreBody{}, err
		}
		b, err := WrappedAnisotropicStationLoftFromSolid(ctx, local, "", unit, o)
		if err != nil {
			return WrappedAnisotropicStationBoreBody{}, err
		}
		if i == 0 {
			out.Outer = b
		} else {
			out.Inner = b
		}
	}
	return out, nil
}
