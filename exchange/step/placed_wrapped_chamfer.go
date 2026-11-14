package step

import (
	"bytes"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedRimChamferBody describes a cylindrical body with one or two equal
// radial/axial rim setbacks. Axis is a positive world coordinate direction.
type WrappedRimChamferBody struct {
	Name           string
	Center, Axis   geom.Vec3
	Radius, Height float64
	Setbacks       [2]float64
	LengthUnit     float64
}

func (p WrappedRimChamferBody) valid() bool {
	return finiteOblique(p.Radius) && finiteOblique(p.Height) &&
		finiteOblique(p.Setbacks[0]) && finiteOblique(p.Setbacks[1]) &&
		p.Height > 0 && min(p.Setbacks[0], p.Setbacks[1]) >= 0 &&
		max(p.Setbacks[0], p.Setbacks[1]) > 0 && max(p.Setbacks[0], p.Setbacks[1]) < p.Radius &&
		p.Setbacks[0]+p.Setbacks[1] < p.Height
}

// WritePlacedWrappedRimChamfer exchanges the exact ruled frustum bands and
// cylindrical middle that bound a circular chamfer.
func WritePlacedWrappedRimChamfer(w io.Writer, p WrappedRimChamferBody) error {
	if w == nil || !p.valid() {
		return geom.ErrDomain
	}
	placement, err := wrappedChartPlacement(p.Axis)
	if err != nil {
		return err
	}
	c := placement.ToLocal(p.Center)
	m := WrappedStationLoftBody{Name: p.Name, LengthUnit: p.LengthUnit, Center: geom.Vec2{X: c.X, Y: c.Y}}
	m.Stations = []trimmed.WrappedLoftStation{{Z: c.Z, Radius: p.Radius - p.Setbacks[0]}}
	if p.Setbacks[0] > 0 {
		m.Stations = append(m.Stations, trimmed.WrappedLoftStation{Z: c.Z + p.Setbacks[0], Radius: p.Radius})
	}
	if p.Setbacks[1] > 0 {
		m.Stations = append(m.Stations, trimmed.WrappedLoftStation{Z: c.Z + (p.Height - p.Setbacks[1]), Radius: p.Radius})
	}
	m.Stations = append(m.Stations, trimmed.WrappedLoftStation{Z: c.Z + p.Height, Radius: p.Radius - p.Setbacks[1]})
	return writePlacedWrappedChart(w, p.Axis, func(out io.Writer) error {
		return WriteWrappedStationLoft(out, m)
	})
}

// ReadPlacedWrappedRimChamfer restricts the fully proved station loft to a
// cylindrical middle and the one or two matching equal-setback end bands.
func ReadPlacedWrappedRimChamfer(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedRimChamferBody, error) {
	var p WrappedRimChamferBody
	raw, placement, err := readPlacedWrappedChart(r, l)
	if err != nil {
		return p, err
	}
	m, err := ReadWrappedStationLoft(bytes.NewReader(raw), l, o)
	if err != nil {
		return p, err
	}
	stations := m.Stations
	if len(stations) != 3 && len(stations) != 4 {
		return p, geom.ErrDomain
	}
	left, right := stations[0], stations[len(stations)-1]
	p = WrappedRimChamferBody{Name: m.Name, Axis: placement.ZAxis(), Radius: stations[1].Radius,
		Center: placement.ToWorld(geom.Vec3{X: m.Center.X, Y: m.Center.Y, Z: left.Z}),
		Height: right.Z - left.Z, LengthUnit: m.LengthUnit}
	p.Setbacks = [2]float64{p.Radius - left.Radius, p.Radius - right.Radius}
	if !p.valid() {
		return WrappedRimChamferBody{}, geom.ErrDomain
	}
	i := 0
	if p.Setbacks[0] > 0 {
		i++
	}
	j := len(stations) - 1
	if p.Setbacks[1] > 0 {
		j--
	}
	if i+1 != j || stations[i].Radius != p.Radius || stations[j].Radius != p.Radius ||
		stations[i].Z != left.Z+p.Setbacks[0] || stations[j].Z != left.Z+(p.Height-p.Setbacks[1]) {
		return WrappedRimChamferBody{}, geom.ErrDomain
	}
	return p, nil
}
