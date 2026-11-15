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

// PlanarInsetIGESOptions bounds the original file and the exact inset proof.
type PlanarInsetIGESOptions struct {
	Limits Limits
	Proof  collision.PlanarFaceContactOptions
}

// PlanarInsetContactPair retains a host and tool as separate original IGES
// bodies. The contact ledger retains all complete original tool footprints.
type PlanarInsetContactPair struct {
	GroupID    int
	LengthUnit float64
	Bodies     [2]PlanarAssemblyBody
	Contact    collision.PlanarInsetContactSet
}

// ReadPlanarInsetContactPair proves the first body is a host meeting complete
// disjoint original faces of the second body only in one host face interior.
func ReadPlanarInsetContactPair(ctx context.Context, r io.Reader, o PlanarInsetIGESOptions) (PlanarInsetContactPair, error) {
	var zero PlanarInsetContactPair
	m, err := readIGESFaceContactBodies(ctx, r, o.Limits, o.Proof.Validation)
	if err != nil {
		return zero, err
	}
	p, err := collision.DiscoverPlanarInsetContactSet(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return zero, err
	}
	return PlanarInsetContactPair{m.GroupID, m.LengthUnit, [2]PlanarAssemblyBody(m.Bodies), p}, nil
}

// WritePlanarInsetContactPair publishes neither original IGES body until
// both original graphs and the complete inset proof reopen successfully.
func WritePlanarInsetContactPair(ctx context.Context, w io.Writer, parts [2]brep.Solid, unit float64, o PlanarInsetIGESOptions) (collision.PlanarInsetContactSet, error) {
	var zero collision.PlanarInsetContactSet
	if ctx == nil || w == nil {
		return zero, geom.ErrInvalid
	}
	p, err := collision.DiscoverPlanarInsetContactSet(ctx, parts[0], parts[1], o.Proof)
	if err != nil {
		return zero, err
	}
	raw, faces, err := authorPlanarIGES(ctx, parts[:], unit, o.Limits, o.Proof.Validation)
	if err != nil {
		return zero, err
	}
	reopened, err := ReadPlanarInsetContactPair(ctx, bytes.NewReader(raw), o)
	if err != nil {
		return zero, fmt.Errorf("iges planar inset readback: %w", err)
	}
	q := reopened.Contact
	if reopened.LengthUnit != unit || p.HostFace != q.HostFace || p.SignedVolume.Cmp(q.SignedVolume) != 0 || len(p.Tools) != len(q.Tools) {
		return zero, igesPlanarFailure(reopened.GroupID, "readback changed exact inset material or units")
	}
	for i, a := range p.Tools {
		b := q.Tools[i]
		if a.ToolFace != b.ToolFace || a.DropAxis != b.DropAxis ||
			a.TwiceHostArea.Cmp(b.TwiceHostArea) != 0 || a.TwiceToolArea.Cmp(b.TwiceToolArea) != 0 ||
			!slices.Equal(a.ToolVertices, b.ToolVertices) || !slices.Equal(a.ToolEdges, b.ToolEdges) {
			return zero, igesPlanarFailure(reopened.GroupID, "readback changed original tool footprint")
		}
	}
	for i, s := range parts {
		if !samePlanarIGESBody(s, reopened.Bodies[i].Solid, faces[i]) {
			return zero, igesPlanarFailure(reopened.Bodies[i].SolidID, "readback changed original inset boundary")
		}
	}
	if err := publishPlanarIGES(ctx, w, raw); err != nil {
		return zero, err
	}
	return p, nil
}
