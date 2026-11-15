package step

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"plainsolid/collision"
	"plainsolid/geom"
)

// FaceContactSTEPOptions bounds exchange and exact surface contact discovery.
type FaceContactSTEPOptions struct {
	Limits Limits
	Proof  collision.PlanarFaceDiscoveryOptions
}

// ReadFaceContactPair imports two separate original planar STEP bodies whose
// only intersection is one common complete face, with matching inner wires.
func ReadFaceContactPair(ctx context.Context, r io.Reader, o FaceContactSTEPOptions) (Model, collision.PlanarFaceContact, error) {
	var zero collision.PlanarFaceContact
	if ctx == nil || r == nil {
		return Model{}, zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Model{}, zero, err
	}
	if o.Limits.MaxSolids == 0 || o.Limits.MaxSolids > 2 {
		o.Limits.MaxSolids = 2
	}
	m, err := readPlanarModel(r, o.Limits, true, false)
	if err != nil {
		return Model{}, zero, err
	}
	if len(m.Bodies) != 2 {
		return Model{}, zero, fmt.Errorf("%w: STEP face contact requires two separate bodies", collision.ErrInvalid)
	}
	if err := checkVertexContactSTEPFaces(ctx, m); err != nil {
		return Model{}, zero, err
	}
	p, err := collision.DiscoverPlanarFaceContact(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return Model{}, zero, err
	}
	return m, p, nil
}

// WriteFaceContactPair writes separate advanced BRep solids and verifies all
// original boundaries and their face contact before publishing the file.
func WriteFaceContactPair(ctx context.Context, w io.Writer, m Model, o FaceContactSTEPOptions) (collision.PlanarFaceContact, error) {
	var zero collision.PlanarFaceContact
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
	p, err := collision.DiscoverPlanarFaceContact(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return zero, err
	}
	out := vertexContactSTEPBuffer{max: l.MaxBytes}
	if err := writePlanarModel(&out, m, true, true); err != nil {
		return zero, err
	}
	got, q, err := ReadFaceContactPair(ctx, bytes.NewReader(out.Bytes()), o)
	if err != nil {
		return zero, fmt.Errorf("step face contact readback: %w", err)
	}
	if got.LengthUnit != m.LengthUnit || p.FaceA != q.FaceA || p.FaceB != q.FaceB ||
		p.DropAxis != q.DropAxis || p.SignedVolume.Cmp(q.SignedVolume) != 0 || p.TwiceProjectedArea.Cmp(q.TwiceProjectedArea) != 0 {
		return zero, fmt.Errorf("step: readback changed original face contact or units")
	}
	for i, b := range m.Bodies {
		if got.Bodies[i].Name != b.Name || !sameExactPlanarSTEPBoundary(b.Solid, got.Bodies[i].Solid) {
			return zero, fmt.Errorf("step: readback changed original face-contact body %d", i)
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	n, err := w.Write(out.Bytes())
	if err != nil {
		return zero, err
	}
	if n != out.Len() {
		return zero, io.ErrShortWrite
	}
	return p, nil
}
