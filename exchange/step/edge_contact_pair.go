package step

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// EdgeContactSTEPOptions bounds exact file transfer separately from the
// proof of original edge-only contact between the two bodies.
type EdgeContactSTEPOptions struct {
	Limits Limits
	Proof  collision.PlanarEdgeContactOptions
}

// ReadEdgeContactPair imports two independent planar STEP bodies with exactly
// one complete original shared edge. It proves original supports and
// physical units without rounding, and keeps separate native topology.
func ReadEdgeContactPair(ctx context.Context, r io.Reader, o EdgeContactSTEPOptions) (Model, collision.PlanarEdgeContact, error) {
	return readSTEPPlanarEdgePair(ctx, r, o, false)
}

// ReadEdgeIntervalContactPair proves the only contact is a positive common
// interval of one original edge from each separate body. It retains both
// exact directed intervals without splitting or welding either source edge.
func ReadEdgeIntervalContactPair(ctx context.Context, r io.Reader, o EdgeContactSTEPOptions) (Model, collision.PlanarEdgeContact, error) {
	return readSTEPPlanarEdgePair(ctx, r, o, true)
}

func readSTEPPlanarEdgePair(ctx context.Context, r io.Reader, o EdgeContactSTEPOptions, interval bool) (Model, collision.PlanarEdgeContact, error) {
	if ctx == nil || r == nil {
		return Model{}, collision.PlanarEdgeContact{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Model{}, collision.PlanarEdgeContact{}, err
	}
	if o.Limits.MaxSolids == 0 || o.Limits.MaxSolids > 2 {
		o.Limits.MaxSolids = 2
	}
	m, err := readPlanarModel(r, o.Limits, true, false)
	if err != nil {
		return Model{}, collision.PlanarEdgeContact{}, err
	}
	if len(m.Bodies) != 2 {
		return Model{}, collision.PlanarEdgeContact{}, fmt.Errorf("step: exact edge contact requires two separate planar bodies")
	}
	if err := checkVertexContactSTEPFaces(ctx, m); err != nil {
		return Model{}, collision.PlanarEdgeContact{}, err
	}
	proof, err := proveSTEPPlanarEdgePair(ctx, m, o.Proof, interval)
	if err != nil {
		return Model{}, collision.PlanarEdgeContact{}, err
	}
	return m, proof, nil
}

// WriteEdgeContactPair writes two separate standard advanced STEP bodies
// and verifies the same edge-only contact before publishing any bytes.
func WriteEdgeContactPair(ctx context.Context, w io.Writer, m Model, o EdgeContactSTEPOptions) (collision.PlanarEdgeContact, error) {
	return writeSTEPPlanarEdgePair(ctx, w, m, o, false)
}

// WriteEdgeIntervalContactPair transfers two original STEP boundaries whose
// sole contact is one positive interval, with full strict readback.
func WriteEdgeIntervalContactPair(ctx context.Context, w io.Writer, m Model, o EdgeContactSTEPOptions) (collision.PlanarEdgeContact, error) {
	return writeSTEPPlanarEdgePair(ctx, w, m, o, true)
}

func proveSTEPPlanarEdgePair(ctx context.Context, m Model, o collision.PlanarEdgeContactOptions, interval bool) (collision.PlanarEdgeContact, error) {
	a, b := m.Bodies[0].Solid, m.Bodies[1].Solid
	if interval {
		return collision.DiscoverPlanarEdgeIntervalContact(ctx, a, b, o)
	}
	return collision.DiscoverPlanarEdgeContact(ctx, a, b, o)
}

func writeSTEPPlanarEdgePair(ctx context.Context, w io.Writer, m Model, o EdgeContactSTEPOptions, interval bool) (collision.PlanarEdgeContact, error) {
	var zero collision.PlanarEdgeContact
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
	proof, err := proveSTEPPlanarEdgePair(ctx, m, o.Proof, interval)
	if err != nil {
		return zero, err
	}
	out := vertexContactSTEPBuffer{max: l.MaxBytes}
	if err := writePlanarModel(&out, m, true, true); err != nil {
		return zero, err
	}
	got, p, err := readSTEPPlanarEdgePair(ctx, bytes.NewReader(out.Bytes()), o, interval)
	if err != nil {
		return zero, fmt.Errorf("step edge contact reopen: %w", err)
	}
	sameEdge := p.Start == proof.Start && p.End == proof.End || p.Start == proof.End && p.End == proof.Start
	if got.LengthUnit != m.LengthUnit || proof.SignedVolume.Cmp(p.SignedVolume) != 0 || !sameEdge {
		return zero, fmt.Errorf("step: readback changed exact edge-contact material or units")
	}
	for i, b := range m.Bodies {
		if got.Bodies[i].Name != b.Name || !sameExactPlanarSTEPBoundary(b.Solid, got.Bodies[i].Solid) {
			return zero, fmt.Errorf("step: readback changed exact edge-contact body %d", i)
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
	return proof, nil
}

func sameExactPlanarSTEPBoundary(a, b brep.Solid) bool {
	if a.Topology() != b.Topology() {
		return false
	}
	av, ae, af := a.Vertices(), a.Edges(), a.Faces()
	bv, be, bf := b.Vertices(), b.Edges(), b.Faces()
	sameLoop := func(a, b []brep.EdgeUse) bool {
		if len(a) != len(b) || len(a) == 0 {
			return false
		}
		first := av[vertexContactLoopVertex(a[0], ae)]
		for i, use := range b {
			if bv[vertexContactLoopVertex(use, be)] != first {
				continue
			}
			for j, use := range a {
				if av[vertexContactLoopVertex(use, ae)] != bv[vertexContactLoopVertex(b[(i+j)%len(b)], be)] {
					return false
				}
			}
			return true
		}
		return false
	}
	for i, f := range af {
		g := bf[i]
		if len(f.Holes) != len(g.Holes) || !sameLoop(f.Boundary, g.Boundary) {
			return false
		}
		for i, h := range f.Holes {
			if !sameLoop(h, g.Holes[i]) {
				return false
			}
		}
	}
	return true
}
