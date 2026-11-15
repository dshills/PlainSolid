package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// FaceContactIGESOptions bounds exchange and exact face contact discovery.
type FaceContactIGESOptions struct {
	Limits Limits
	Proof  collision.PlanarFaceDiscoveryOptions
}

// PlanarFaceContactPair keeps both original material roots and the complete
// face, vertex and edge incidences they share geometrically.
type PlanarFaceContactPair struct {
	GroupID    int
	LengthUnit float64
	Bodies     [2]PlanarAssemblyBody
	Contact    collision.PlanarFaceContact
}

// ReadPlanarFaceContactPair imports two separate solids in an IGES flat group.
// It certifies one complete shared original face, including its inner wires.
func ReadPlanarFaceContactPair(ctx context.Context, r io.Reader, o FaceContactIGESOptions) (PlanarFaceContactPair, error) {
	var zero PlanarFaceContactPair
	m, err := readIGESFaceContactBodies(ctx, r, o.Limits, o.Proof.Contact.Validation)
	if err != nil {
		return zero, err
	}
	bodies := [2]PlanarAssemblyBody(m.Bodies)
	p, err := collision.DiscoverPlanarFaceContact(ctx, bodies[0].Solid, bodies[1].Solid, o.Proof)
	if err != nil {
		return zero, err
	}
	return PlanarFaceContactPair{GroupID: m.GroupID, LengthUnit: m.LengthUnit, Bodies: bodies, Contact: p}, nil
}

func readIGESFaceContactBodies(ctx context.Context, r io.Reader, limits Limits, validation collision.LocationOptions) (PlanarVertexContactAssembly, error) {
	if ctx == nil || r == nil {
		return PlanarVertexContactAssembly{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return PlanarVertexContactAssembly{}, err
	}
	d, err := readPlanarTopologySource(r, limits)
	if err != nil {
		return PlanarVertexContactAssembly{}, err
	}
	m, err := readIGESPlanarAssemblyBodies(ctx, d, collision.VertexContactAssemblyOptions{MaxSolids: 2, Validation: validation})
	if err != nil {
		return PlanarVertexContactAssembly{}, err
	}
	return m, nil
}

// WritePlanarFaceContactPair preserves the separate original IGES root
// topology and re-proves the face contact before publishing any bytes.
func WritePlanarFaceContactPair(ctx context.Context, w io.Writer, parts [2]brep.Solid, unit float64, o FaceContactIGESOptions) (collision.PlanarFaceContact, error) {
	var zero collision.PlanarFaceContact
	if ctx == nil || w == nil {
		return zero, geom.ErrInvalid
	}
	p, err := collision.DiscoverPlanarFaceContact(ctx, parts[0], parts[1], o.Proof)
	if err != nil {
		return zero, err
	}
	raw, faces, err := authorPlanarIGES(ctx, parts[:], unit, o.Limits, o.Proof.Contact.Validation)
	if err != nil {
		return zero, err
	}
	got, err := ReadPlanarFaceContactPair(ctx, bytes.NewReader(raw), o)
	if err != nil {
		return zero, fmt.Errorf("iges planar face contact readback: %w", err)
	}
	q := got.Contact
	if got.LengthUnit != unit || p.DropAxis != q.DropAxis ||
		p.TwiceProjectedArea.Cmp(q.TwiceProjectedArea) != 0 || p.SignedVolume.Cmp(q.SignedVolume) != 0 {
		return zero, igesPlanarFailure(got.GroupID, "readback changed exact face contact or units")
	}
	for i, s := range parts {
		oldID, newID := [2]int{p.FaceA, p.FaceB}[i], [2]int{q.FaceA, q.FaceB}[i]
		if !samePlanarIGESBody(s, got.Bodies[i].Solid, faces[i]) || !reflect.DeepEqual(s.Faces()[oldID], faces[i][newID]) {
			return zero, igesPlanarFailure(got.Bodies[i].SolidID, "readback changed exact original face-contact topology")
		}
	}
	if err := publishPlanarIGES(ctx, w, raw); err != nil {
		return zero, err
	}
	return p, nil
}
