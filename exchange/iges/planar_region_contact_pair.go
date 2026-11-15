package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// PlanarRegionIGESOptions bounds file parsing and the exact contact proof.
type PlanarRegionIGESOptions struct {
	Limits Limits
	Proof  collision.PlanarFaceContactOptions
}

// PlanarRegionContactPair retains the independent original IGES solids and
// their shared region even when the original face divisions differ.
type PlanarRegionContactPair struct {
	GroupID    int
	LengthUnit float64
	Bodies     [2]PlanarAssemblyBody
	Contact    collision.PlanarRegionContact
}

// ReadPlanarRegionContactPair proves a complete common region and matching
// seams between two separately indexed IGES material roots.
func ReadPlanarRegionContactPair(ctx context.Context, r io.Reader, o PlanarRegionIGESOptions) (PlanarRegionContactPair, error) {
	var zero PlanarRegionContactPair
	m, err := readIGESFaceContactBodies(ctx, r, o.Limits, o.Proof.Validation)
	if err != nil {
		return zero, err
	}
	p, err := collision.DiscoverPlanarRegionContact(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return zero, err
	}
	return PlanarRegionContactPair{GroupID: m.GroupID, LengthUnit: m.LengthUnit, Bodies: [2]PlanarAssemblyBody(m.Bodies), Contact: p}, nil
}

// WritePlanarRegionContactPair emits separate original IGES topology and
// re-proves the complete shared region before publishing any bytes.
func WritePlanarRegionContactPair(ctx context.Context, w io.Writer, parts [2]brep.Solid, unit float64, o PlanarRegionIGESOptions) (collision.PlanarRegionContact, error) {
	var zero collision.PlanarRegionContact
	if ctx == nil || w == nil {
		return zero, geom.ErrInvalid
	}
	p, err := collision.DiscoverPlanarRegionContact(ctx, parts[0], parts[1], o.Proof)
	if err != nil {
		return zero, err
	}
	raw, faces, err := authorPlanarIGES(ctx, parts[:], unit, o.Limits, o.Proof.Validation)
	if err != nil {
		return zero, err
	}
	got, err := ReadPlanarRegionContactPair(ctx, bytes.NewReader(raw), o)
	if err != nil {
		return zero, fmt.Errorf("iges planar region contact readback: %w", err)
	}
	q := got.Contact
	if got.LengthUnit != unit || q.SignedVolume.Cmp(p.SignedVolume) != 0 ||
		!slices.Equal(p.Vertices, q.Vertices) || !slices.Equal(p.Edges, q.Edges) {
		return zero, igesPlanarFailure(got.GroupID, "readback changed exact original region contact or units")
	}
	for i, s := range parts {
		if !samePlanarIGESBody(s, got.Bodies[i].Solid, faces[i]) ||
			!slices.Equal(p.Faces[i], q.Faces[i]) || !slices.Equal(p.DropAxes[i], q.DropAxes[i]) {
			return zero, igesPlanarFailure(got.Bodies[i].SolidID, "readback changed exact original region owner")
		}
		for j, a := range p.TwiceProjectedAreas[i] {
			if a.Cmp(q.TwiceProjectedAreas[i][j]) != 0 {
				return zero, igesPlanarFailure(got.Bodies[i].SolidID, "readback changed exact original face area")
			}
		}
	}
	if err := publishPlanarIGES(ctx, w, raw); err != nil {
		return zero, err
	}
	return p, nil
}
