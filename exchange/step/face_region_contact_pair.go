package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"

	"plainsolid/collision"
	"plainsolid/geom"
)

// PlanarRegionSTEPOptions bounds original STEP parsing and the exact material
// proof for a contact region with potentially different face subdivisions.
type PlanarRegionSTEPOptions struct {
	Limits Limits
	Proof  collision.PlanarFaceContactOptions
}

// ReadPlanarRegionContactPair retains two original STEP bodies whose only
// contact is a pair of complete original regions with identical exterior seams.
func ReadPlanarRegionContactPair(ctx context.Context, r io.Reader, o PlanarRegionSTEPOptions) (Model, collision.PlanarRegionContact, error) {
	var zero collision.PlanarRegionContact
	m, err := readExactSTEPFaceContactBodies(ctx, r, o.Limits)
	if err != nil {
		return Model{}, zero, err
	}
	p, err := collision.DiscoverPlanarRegionContact(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return Model{}, zero, err
	}
	return m, p, nil
}

// WritePlanarRegionContactPair writes two separate original STEP BReps and
// re-proves their full shared region before publishing any output.
func WritePlanarRegionContactPair(ctx context.Context, w io.Writer, m Model, o PlanarRegionSTEPOptions) (collision.PlanarRegionContact, error) {
	var zero collision.PlanarRegionContact
	if ctx == nil || w == nil || len(m.Bodies) != 2 {
		return zero, geom.ErrInvalid
	}
	l, err := o.Limits.defaults()
	if err != nil {
		return zero, err
	}
	if l.MaxSolids < 2 {
		return zero, collision.ErrLimit
	}
	if err := checkVertexContactSTEPFaces(ctx, m); err != nil {
		return zero, err
	}
	p, err := collision.DiscoverPlanarRegionContact(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return zero, err
	}
	raw := vertexContactSTEPBuffer{max: l.MaxBytes}
	if err := writePlanarModel(&raw, m, true, true); err != nil {
		return zero, err
	}
	after, q, err := ReadPlanarRegionContactPair(ctx, bytes.NewReader(raw.Bytes()), o)
	if err != nil {
		return zero, fmt.Errorf("step region contact readback: %w", err)
	}
	if after.LengthUnit != m.LengthUnit || p.SignedVolume.Cmp(q.SignedVolume) != 0 ||
		len(p.Edges) != len(q.Edges) || len(p.Vertices) != len(q.Vertices) {
		return zero, fmt.Errorf("step: readback changed exact region contact topology, material or units")
	}
	for i, b := range m.Bodies {
		if after.Bodies[i].Name != b.Name || !sameExactPlanarSTEPBoundary(b.Solid, after.Bodies[i].Solid) ||
			!slices.Equal(p.Faces[i], q.Faces[i]) || !slices.Equal(p.DropAxes[i], q.DropAxes[i]) {
			return zero, fmt.Errorf("step: readback changed original region owner %d", i)
		}
		for j, a := range p.TwiceProjectedAreas[i] {
			if a.Cmp(q.TwiceProjectedAreas[i][j]) != 0 {
				return zero, fmt.Errorf("step: readback changed original region owner area")
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	n, err := w.Write(raw.Bytes())
	if err != nil {
		return zero, err
	}
	if n != raw.Len() {
		return zero, io.ErrShortWrite
	}
	return p, nil
}
