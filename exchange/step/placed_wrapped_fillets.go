package step

import (
	"bytes"
	"io"

	"plainsolid/brep/trimmed"

	"plainsolid/geom"
)

// WritePlacedWrappedRimFillet places the wrapped rim fillet along one positive
// coordinate axis. Center is in world coordinates. Wall charts are provided
// by the canonical constructor, so callers leave Walls empty.
func WritePlacedWrappedRimFillet(w io.Writer, m WrappedRimFilletBody, axis geom.Vec3) error {
	if w == nil || len(m.Walls) != 0 {
		return geom.ErrInvalid
	}
	placement, err := wrappedChartPlacement(axis)
	if err != nil {
		return err
	}
	if axis.Z == 1 {
		return WriteWrappedRimFillet(w, m)
	}
	m.Center = placement.ToLocal(m.Center)
	return writePlacedWrappedChart(w, axis, func(raw io.Writer) error {
		return WriteWrappedRimFillet(raw, m)
	})
}

func writePlacedWrappedChart(w io.Writer, axis geom.Vec3, writeCanonical func(io.Writer) error) error {
	if axis.Z == 1 {
		return writeCanonical(w)
	}
	var raw bytes.Buffer
	if err := writeCanonical(&raw); err != nil {
		return err
	}
	f, err := Parse(bytes.NewReader(raw.Bytes()), Limits{})
	if err != nil {
		return err
	}
	if err := rotateWrappedChartGraph(f, axis, false); err != nil {
		return err
	}
	out := serializeWrappedProofFile(f)
	n, err := w.Write(out)
	if err == nil && n != len(out) {
		return io.ErrShortWrite
	}
	return err
}

// ReadPlacedWrappedRimFillet recovers a world-space center and positive
// coordinate axis. It moves the entire graph to the canonical chart before
// applying the existing whole-surface and topology proof. The returned Solid
// and Walls are left empty for a world-space result.
func ReadPlacedWrappedRimFillet(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedRimFilletBody, geom.Vec3, error) {
	raw, placement, err := readPlacedWrappedChart(r, l)
	if err != nil {
		return WrappedRimFilletBody{}, geom.Vec3{}, err
	}
	p, err := ReadWrappedRimFillet(bytes.NewReader(raw), l, o)
	if err != nil {
		return WrappedRimFilletBody{}, geom.Vec3{}, err
	}
	p.Center = placement.ToWorld(p.Center)
	p.Solid = trimmed.Solid{}
	p.Walls = nil
	return p, placement.ZAxis(), nil
}

func readPlacedWrappedChart(r io.Reader, l Limits) ([]byte, geom.Frame, error) {
	f, err := Parse(r, l)
	if err != nil {
		return nil, geom.Frame{}, err
	}
	var axis geom.Vec3
	for _, id := range f.Order {
		if _, e := simple(f, id, "PLANE", 2); e != nil {
			continue
		}
		plane, e := planeFrame(f, id)
		if e != nil {
			return nil, geom.Frame{}, e
		}
		z := plane.ZAxis()
		if z.X < 0 || z.Y < 0 || z.Z < 0 {
			z = z.Scale(-1)
		}
		if axis != (geom.Vec3{}) && axis != z {
			return nil, geom.Frame{}, geom.ErrDomain
		}
		axis = z
	}
	placement, err := wrappedChartPlacement(axis)
	if err != nil {
		return nil, geom.Frame{}, err
	}
	if axis.Z != 1 {
		if err := rotateWrappedChartGraph(f, axis, true); err != nil {
			return nil, geom.Frame{}, err
		}
	}
	return serializeWrappedProofFile(f), placement, nil
}

func wrappedChartPlacement(axis geom.Vec3) (geom.Frame, error) {
	switch axis {
	case geom.Vec3{X: 1}:
		return geom.NewFrame(geom.Vec3{}, geom.Vec3{Y: 1}, axis)
	case geom.Vec3{Y: 1}:
		return geom.NewFrame(geom.Vec3{}, geom.Vec3{Z: 1}, axis)
	case geom.Vec3{Z: 1}:
		return geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, axis)
	default:
		return geom.Frame{}, geom.ErrDomain
	}
}

// The preserved graph describes a rigidly placed surface with identical
// parameters. Rotate model-space coordinates and directions, leaving every
// UV, weight, knot, scalar and topology reference exactly as represented.
func rotateWrappedChartGraph(f *File, axis geom.Vec3, inverse bool) error {
	from := [3]int{2, 0, 1}
	if (axis.Y == 1) != inverse {
		from = [3]int{1, 2, 0}
	}
	for _, id := range f.Order {
		ent := f.Entities[id]
		for i, rec := range ent.Records {
			if rec.Type != "CARTESIAN_POINT" && rec.Type != "DIRECTION" {
				continue
			}
			if len(rec.Args) != 2 || rec.Args[1].Kind != Aggregate {
				return geom.ErrDomain
			}
			items := rec.Args[1].Items
			if len(items) == 2 {
				continue
			}
			if len(items) != 3 {
				return geom.ErrDomain
			}
			old := [3]Value{items[0], items[1], items[2]}
			for j, index := range from {
				if old[index].Kind != Number {
					return geom.ErrDomain
				}
				items[j] = old[index]
			}
			ent.Records[i] = rec
		}
		f.Entities[id] = ent
	}
	return nil
}
