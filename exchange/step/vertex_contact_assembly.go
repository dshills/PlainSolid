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

// VertexContactSTEPOptions bounds standard planar STEP transfer and the exact
// assembly proof independently. FacetedWriter selects FACETED_BREP instead of
// the default planar ADVANCED_FACE output; the reader accepts either.
type VertexContactSTEPOptions struct {
	Limits        Limits
	Proof         collision.VertexContactAssemblyOptions
	FacetedWriter bool
}

// ReadVertexContactAssembly interprets separate planar STEP bodies as a
// compound with disjoint interiors and discovers their complete exact vertex
// contact graph. The file requires no private marker or supplementary ledger.
// The ordinary planar STEP transfer limits still apply; mapped occurrences and
// nonplanar geometry are not part of this entry point.
func ReadVertexContactAssembly(ctx context.Context, r io.Reader, o VertexContactSTEPOptions) (Model, collision.VertexContactAssemblyReport, error) {
	var zero Model
	var noReport collision.VertexContactAssemblyReport
	if ctx == nil || r == nil {
		return zero, noReport, collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, noReport, err
	}
	m, err := Read(r, o.Limits)
	if err != nil {
		return zero, noReport, err
	}
	p, err := collision.DiscoverVertexContactAssembly(ctx, vertexContactBodies(m), o.Proof)
	if err != nil {
		return zero, noReport, err
	}
	return m, p, nil
}

// WriteVertexContactAssembly writes separate standard STEP bodies after exact
// compound proof. It independently reopens the generated file under the
// supplied limits and checks units, body order and measures, and the model
// coordinates of every contact before publishing any bytes. The returned
// contact ledger uses original input vertex IDs; a reader derives its own IDs
// from the transferred geometry.
func WriteVertexContactAssembly(ctx context.Context, w io.Writer, m Model, o VertexContactSTEPOptions) (collision.VertexContactAssemblyReport, error) {
	var empty collision.VertexContactAssemblyReport
	if ctx == nil || w == nil || len(m.Bodies) == 0 {
		return empty, collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	limits, err := o.Limits.defaults()
	if err != nil {
		return empty, err
	}
	if len(m.Bodies) > limits.MaxSolids {
		return empty, collision.ErrLimit
	}
	proof, err := collision.DiscoverVertexContactAssembly(ctx, vertexContactBodies(m), o.Proof)
	if err != nil {
		return empty, err
	}
	var staged vertexContactSTEPBuffer
	staged.max = limits.MaxBytes
	if err := writeModel(&staged, m, !o.FacetedWriter); err != nil {
		return empty, err
	}
	reopened, check, err := ReadVertexContactAssembly(ctx, bytes.NewReader(staged.Bytes()), o)
	if err != nil {
		return empty, fmt.Errorf("step compound reopen: %w", err)
	}
	if m.LengthUnit != reopened.LengthUnit || len(m.Bodies) != len(reopened.Bodies) || !sameSTEPContactGeometry(m, proof, reopened, check) {
		return empty, fmt.Errorf("%w: STEP component transfer changed material or contact geometry", collision.ErrInvalid)
	}
	for i, body := range m.Bodies {
		if body.Name != reopened.Bodies[i].Name || body.Solid.Topology() != reopened.Bodies[i].Solid.Topology() {
			return empty, fmt.Errorf("%w: STEP component %d changed name or topology", collision.ErrInvalid, i)
		}
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	n, err := w.Write(staged.Bytes())
	if err != nil {
		return empty, err
	}
	if n != staged.Len() {
		return empty, io.ErrShortWrite
	}
	return proof, nil
}

type vertexContactSTEPBuffer struct {
	bytes.Buffer
	max int64
}

func (b *vertexContactSTEPBuffer) Write(p []byte) (int, error) {
	if int64(len(p)) > b.max-int64(b.Len()) {
		return 0, collision.ErrLimit
	}
	return b.Buffer.Write(p)
}

func vertexContactBodies(m Model) []brep.Solid {
	solids := make([]brep.Solid, len(m.Bodies))
	for i, body := range m.Bodies {
		solids[i] = body.Solid
	}
	return solids
}

func sameSTEPContactGeometry(a Model, left collision.VertexContactAssemblyReport, b Model, right collision.VertexContactAssemblyReport) bool {
	if left.SignedVolume.Cmp(right.SignedVolume) != 0 || len(left.Contacts) != len(right.Contacts) {
		return false
	}
	type contact struct {
		pair [2]int
		xyz  geom.Vec3
	}
	geometry := func(m Model, r collision.VertexContactAssemblyReport) map[contact]bool {
		vertices := make([][]geom.Vec3, len(m.Bodies))
		for i, body := range m.Bodies {
			vertices[i] = body.Solid.Vertices()
		}
		out := make(map[contact]bool, len(r.Contacts))
		for _, c := range r.Contacts {
			out[contact{[2]int{c.A.Solid, c.B.Solid}, vertices[c.A.Solid][c.A.Vertex]}] = true
		}
		return out
	}
	old, next := geometry(a, left), geometry(b, right)
	for vertex := range old {
		if !next[vertex] {
			return false
		}
	}
	return true
}
