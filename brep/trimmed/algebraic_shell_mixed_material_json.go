package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type nativeMixedMaterialVertex struct {
	Operand, Original int
	Section           bool
}
type nativeMixedMaterial struct {
	Version   int                          `json:"version"`
	Kind      string                       `json:"kind"`
	Operation parametric.BooleanOp         `json:"operation"`
	Section   json.RawMessage              `json:"section"`
	Topology  AlgebraicSubdivisionTopology `json:"topology"`
	Faces     []AlgebraicShellBooleanFace  `json:"faces"`
	Edges     []AlgebraicShellBooleanEdge  `json:"edges"`
	Vertices  []nativeMixedMaterialVertex  `json:"vertices"`
}

func mixedMaterialWitness(s *MixedAlgebraicShellBoolean) nativeMixedMaterial {
	q := nativeMixedMaterial{Version: 1, Kind: "trimmed-mixed-algebraic-material", Operation: s.boundary.op, Topology: s.boundary.Topology, Faces: s.Faces(), Edges: s.Edges()}
	for _, v := range s.boundary.vertices {
		q.Vertices = append(q.Vertices, nativeMixedMaterialVertex{v.Operand, v.OriginalSubdivisionVertex, v.SectionPoint != nil})
	}
	return q
}
func mixedMaterialEntities(q nativeMixedMaterial) int {
	n := len(q.Faces) + len(q.Edges) + len(q.Vertices)
	for _, f := range q.Faces {
		n += len(f.Outer) + len(f.Holes)
		for _, h := range f.Holes {
			n += len(h)
		}
	}
	return n
}
func mixedSectionSourceEntities(s *MixedAlgebraicShellSection) int {
	n := len(s.source.branches)
	for _, body := range [2]Solid{s.source.a, s.source.b} {
		n += len(body.faces) + len(body.edges) + len(body.vertices)
	}
	for _, p := range s.source.sections {
		for _, t := range p.recipes {
			n += len(t.Cells)
		}
	}
	for _, p := range s.proposals {
		n += len(p.Pieces)
	}
	for _, w := range s.source.wires {
		n += len(w)
	}
	return n
}

// WriteMixedAlgebraicShellMaterialJSON persists original source history and
// exact restrictions, plus output ownership witnesses. Witnesses never replace
// reconstruction. The default 32 MiB/20000-entity payload is fully prepared
// before any bytes are written.
func WriteMixedAlgebraicShellMaterialJSON(w io.Writer, s *MixedAlgebraicShellBoolean) error {
	if w == nil || s == nil || s.section == nil || s.section.source == nil {
		return geom.ErrInvalid
	}
	q := mixedMaterialWitness(s)
	if mixedMaterialEntities(q)+mixedSectionSourceEntities(s.section) > 20000 {
		return geom.ErrConvergence
	}
	var section bytes.Buffer
	if err := WriteMixedAlgebraicShellSectionJSON(&section, s.section); err != nil {
		return err
	}
	q.Section = section.Bytes()
	data, err := json.Marshal(q)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > 32<<20 {
		return geom.ErrConvergence
	}
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

// ReadMixedAlgebraicShellMaterialJSON rebuilds complete original-pair coverage,
// both imprints, material classification and closed incidence before comparing
// the stored operation's output ownership. All failure paths return nil.
func ReadMixedAlgebraicShellMaterialJSON(ctx context.Context, r io.Reader, l JSONLimits, section MixedAlgebraicShellSectionOptions, o AlgebraicShellBooleanOptions) (*MixedAlgebraicShellBoolean, error) {
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxEntities < 0 || l.MaxControlPoints < 0 {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	if l.MaxEntities == 0 {
		l.MaxEntities = 20000
	}
	data, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > l.MaxBytes {
		return nil, geom.ErrConvergence
	}
	var q nativeMixedMaterial
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&q); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF || q.Version != 1 || q.Kind != "trimmed-mixed-algebraic-material" || q.Operation > parametric.Difference || len(q.Section) == 0 {
		return nil, geom.ErrInvalid
	}
	n := mixedMaterialEntities(q)
	if n >= l.MaxEntities {
		return nil, geom.ErrConvergence
	}
	l.MaxEntities -= n
	source, err := ReadMixedAlgebraicShellSectionJSON(ctx, bytes.NewReader(q.Section), l, section)
	if err != nil {
		return nil, err
	}
	bodies, err := source.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{q.Operation}, o)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	want := mixedMaterialWitness(bodies[0])
	q.Section = nil
	if !reflect.DeepEqual(q, want) {
		return nil, geom.ErrDomain
	}
	return bodies[0], nil
}
