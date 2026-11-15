package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// EdgeContactIGESOptions bounds the file and the separate material proof.
type EdgeContactIGESOptions struct {
	Limits Limits
	Proof  collision.PlanarEdgeContactOptions
}

// PlanarEdgeContactPair retains the original group and separate edge owners.
type PlanarEdgeContactPair struct {
	GroupID    int
	LengthUnit float64
	Bodies     [2]PlanarAssemblyBody
	Contact    collision.PlanarEdgeContact
}

// ReadPlanarEdgeContactPair proves that two separate planar IGES bodies
// meet only on one entire edge from each original solid. Their records
// remain independent even where the exact endpoint coordinates coincide.
func ReadPlanarEdgeContactPair(ctx context.Context, r io.Reader, o EdgeContactIGESOptions) (PlanarEdgeContactPair, error) {
	if ctx == nil {
		return PlanarEdgeContactPair{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return PlanarEdgeContactPair{}, err
	}
	d, err := readPlanarTopologySource(r, o.Limits)
	if err != nil {
		return PlanarEdgeContactPair{}, err
	}
	m, err := readIGESPlanarAssemblyBodies(ctx, d, collision.VertexContactAssemblyOptions{MaxSolids: 2, Validation: o.Proof.Validation})
	if err != nil {
		return PlanarEdgeContactPair{}, err
	}
	proof, err := collision.DiscoverPlanarEdgeContact(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return PlanarEdgeContactPair{}, err
	}
	return PlanarEdgeContactPair{m.GroupID, m.LengthUnit, [2]PlanarAssemblyBody(m.Bodies), proof}, nil
}

// WritePlanarEdgeContactPair emits two independently indexed standard
// material roots and proves the same original edge-only contact on readback.
func WritePlanarEdgeContactPair(ctx context.Context, w io.Writer, parts [2]brep.Solid, unit float64, o EdgeContactIGESOptions) (collision.PlanarEdgeContact, error) {
	var zero collision.PlanarEdgeContact
	if ctx == nil || w == nil {
		return zero, geom.ErrInvalid
	}
	proof, err := collision.DiscoverPlanarEdgeContact(ctx, parts[0], parts[1], o.Proof)
	if err != nil {
		return zero, err
	}
	raw, faces, err := authorPlanarIGES(ctx, parts[:], unit, o.Limits, o.Proof.Validation)
	if err != nil {
		return zero, err
	}
	got, err := ReadPlanarEdgeContactPair(ctx, bytes.NewReader(raw), o)
	if err != nil {
		return zero, fmt.Errorf("iges planar edge contact readback: %w", err)
	}
	p := got.Contact
	if got.LengthUnit != unit || p.EdgeA != proof.EdgeA || p.EdgeB != proof.EdgeB ||
		p.Start != proof.Start || p.End != proof.End || p.Reversed != proof.Reversed || p.SignedVolume.Cmp(proof.SignedVolume) != 0 {
		return zero, igesPlanarFailure(0, "readback changed exact edge contact or units")
	}
	for i, s := range parts {
		if !samePlanarIGESBody(s, got.Bodies[i].Solid, faces[i]) {
			return zero, igesPlanarFailure(got.Bodies[i].SolidID, "readback changed exact native topology")
		}
	}
	if err := publishPlanarIGES(ctx, w, raw); err != nil {
		return zero, err
	}
	return proof, nil
}
