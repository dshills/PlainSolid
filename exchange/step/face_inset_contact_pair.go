package step

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"plainsolid/collision"
	"plainsolid/geom"
)

// PlanarInsetSTEPOptions bounds the original two-body STEP and inset proof.
type PlanarInsetSTEPOptions struct {
	Limits Limits
	Proof  collision.PlanarFaceContactOptions
}

// ReadPlanarInsetContactPair authenticates two separate original bodies.
// The first is the host; the second touches a strict interior of one of its
// faces with one or more disjoint complete original tool faces.
func ReadPlanarInsetContactPair(ctx context.Context, r io.Reader, o PlanarInsetSTEPOptions) (Model, collision.PlanarInsetContactSet, error) {
	var zero collision.PlanarInsetContactSet
	m, err := readExactSTEPFaceContactBodies(ctx, r, o.Limits)
	if err != nil {
		return Model{}, zero, err
	}
	p, err := collision.DiscoverPlanarInsetContactSet(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return Model{}, zero, err
	}
	return m, p, nil
}

// WritePlanarInsetContactPair writes both independent original bodies and
// verifies their full inset interface after reopening the output.
func WritePlanarInsetContactPair(ctx context.Context, w io.Writer, m Model, o PlanarInsetSTEPOptions) (collision.PlanarInsetContactSet, error) {
	var zero collision.PlanarInsetContactSet
	if ctx == nil || w == nil || len(m.Bodies) != 2 {
		return zero, geom.ErrInvalid
	}
	limits, err := o.Limits.defaults()
	if err != nil {
		return zero, err
	}
	if limits.MaxSolids < 2 {
		return zero, collision.ErrLimit
	}
	if err := checkVertexContactSTEPFaces(ctx, m); err != nil {
		return zero, err
	}
	p, err := collision.DiscoverPlanarInsetContactSet(ctx, m.Bodies[0].Solid, m.Bodies[1].Solid, o.Proof)
	if err != nil {
		return zero, err
	}
	raw := vertexContactSTEPBuffer{max: limits.MaxBytes}
	if err := writePlanarModel(&raw, m, true, true); err != nil {
		return zero, err
	}
	reopened, q, err := ReadPlanarInsetContactPair(ctx, bytes.NewReader(raw.Bytes()), o)
	if err != nil {
		return zero, fmt.Errorf("step inset readback: %w", err)
	}
	if reopened.LengthUnit != m.LengthUnit || p.HostFace != q.HostFace || p.SignedVolume.Cmp(q.SignedVolume) != 0 || len(p.Tools) != len(q.Tools) {
		return zero, fmt.Errorf("step: readback changed exact inset material or units")
	}
	for i, a := range p.Tools {
		b := q.Tools[i]
		if a.ToolFace != b.ToolFace || a.DropAxis != b.DropAxis || a.TwiceHostArea.Cmp(b.TwiceHostArea) != 0 || a.TwiceToolArea.Cmp(b.TwiceToolArea) != 0 {
			return zero, fmt.Errorf("step: readback changed original tool face %d", i)
		}
	}
	for i, b := range m.Bodies {
		if b.Name != reopened.Bodies[i].Name || !sameExactPlanarSTEPBoundary(b.Solid, reopened.Bodies[i].Solid) {
			return zero, fmt.Errorf("step: readback changed original inset body %d", i)
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
