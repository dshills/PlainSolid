package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// VertexContactSTEPOptions bounds standard planar STEP transfer and the exact
// assembly proof independently. FacetedWriter selects FACETED_BREP instead of
// the default planar ADVANCED_FACE output; the reader accepts either.
// Proof.AllowCavities also admits planar advanced BREP_WITH_VOIDS.
type VertexContactSTEPOptions struct {
	Limits        Limits
	Proof         collision.VertexContactAssemblyOptions
	FacetedWriter bool
}

// ReadVertexContactAssembly interprets separate planar STEP bodies as a
// compound with disjoint interiors and discovers their complete exact vertex
// contact graph. The file requires no private marker or supplementary ledger.
// Original STEP vertex coordinates must be exactly representable as binary64,
// and each bounded original face polygon must be exactly planar. Declared
// planes and model lines must agree exactly with the boundary. If present,
// both planar edge pcurves must lift exactly in rational owner charts.
// Vertices that would coincide only after rounding cannot acquire a contact.
// Mapped occurrences and nonplanar geometry are outside this entry point.
func ReadVertexContactAssembly(ctx context.Context, r io.Reader, o VertexContactSTEPOptions) (Model, collision.VertexContactAssemblyReport, error) {
	var zero Model
	var noReport collision.VertexContactAssemblyReport
	if ctx == nil || r == nil {
		return zero, noReport, collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, noReport, err
	}
	m, err := readPlanarModel(r, o.Limits, true, o.Proof.AllowCavities)
	if err != nil {
		return zero, noReport, err
	}
	p, err := collision.DiscoverVertexContactAssembly(ctx, vertexContactBodies(m), o.Proof)
	if err != nil {
		return zero, noReport, err
	}
	if err := checkVertexContactSTEPFaces(ctx, m); err != nil {
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
	if err := checkVertexContactSTEPFaces(ctx, m); err != nil {
		return empty, err
	}
	var staged vertexContactSTEPBuffer
	staged.max = limits.MaxBytes
	if err := writeVertexContactModel(ctx, &staged, m, o); err != nil {
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

func exactVertexContactPointNumber(v Value) (float64, error) {
	q, err := exactVertexContactScalar(v)
	if err != nil {
		return 0, err
	}
	return geom.RepresentedExactScalar(q)
}

func exactVertexContactScalar(v Value) (*big.Rat, error) {
	if v.Kind != Number || len(v.Text) > 2500 {
		return nil, geom.ErrUnrepresentable
	}
	if i := strings.IndexAny(v.Text, "eE"); i >= 0 {
		exp, err := strconv.Atoi(v.Text[i+1:])
		if err != nil || exp < -4096 || exp > 4096 {
			return nil, geom.ErrUnrepresentable
		}
	}
	q, ok := new(big.Rat).SetString(v.Text)
	if !ok || q.Num().BitLen() > 8192 || q.Denom().BitLen() > 8192 {
		return nil, geom.ErrUnrepresentable
	}
	return q, nil
}

func exactVertexContactPointText(x float64) string {
	return exactVertexContactDyadicText(new(big.Rat).SetFloat64(x))
}

func exactVertexContactDyadicText(q *big.Rat) string {
	if q.IsInt() {
		return q.Num().String() + "."
	}
	// A dyadic denominator is a power of two, so its decimal terminates.
	return q.FloatString(q.Denom().BitLen() - 1)
}

func checkVertexContactSTEPFaces(ctx context.Context, m Model) error {
	for bodyID, body := range m.Bodies {
		if err := ctx.Err(); err != nil {
			return err
		}
		mesh, faces, edges := body.Solid.Tessellate(), body.Solid.Faces(), body.Solid.Edges()
		planes := make([][3]geom.Vec3, len(faces))
		seen := make([]bool, len(faces))
		for i, face := range body.Solid.TriangleFaces() {
			if seen[face] {
				continue
			}
			seen[face] = true
			t := mesh.Triangles[i]
			planes[face] = [3]geom.Vec3{mesh.Vertices[t[0]], mesh.Vertices[t[1]], mesh.Vertices[t[2]]}
		}
		for i, face := range faces {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !seen[i] {
				return collision.ErrInvalid
			}
			plane := planes[i]
			for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
				for _, u := range loop {
					e := edges[u.Edge]
					vertex := e.Start
					if u.Reversed {
						vertex = e.End
					}
					sign, err := geom.Orient3D(plane[0], plane[1], plane[2], mesh.Vertices[vertex])
					if err != nil || sign != 0 {
						return fmt.Errorf("%w: STEP component %d face %d original vertices are not exactly planar", collision.ErrInvalid, bodyID, i)
					}
				}
			}
		}
	}
	return ctx.Err()
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
