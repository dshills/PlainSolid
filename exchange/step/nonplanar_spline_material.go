package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type NonplanarSplineMaterialKind string

const (
	NonplanarQuadraticMaterial     NonplanarSplineMaterialKind = "quadratic-shell"
	NonplanarCubicMaterial         NonplanarSplineMaterialKind = "cubic-shell"
	NonplanarMultispanUVMaterial   NonplanarSplineMaterialKind = "multispan-uv-shell"
	NonplanarRationalSpeedMaterial NonplanarSplineMaterialKind = "rational-speed-shell"
	NonplanarCavityMaterial        NonplanarSplineMaterialKind = "single-void"
	NonplanarMultipleVoidsMaterial NonplanarSplineMaterialKind = "multiple-voids"
)

// NonplanarSplineMaterialProvenance separates original entity IDs from native
// face indices. Each group follows original outer/void shell order.
type NonplanarSplineMaterialProvenance struct {
	BodyID                                  int64
	ShellIDs, VoidOccurrenceIDs             []int64
	FaceIDs, EdgeIDs, PCurveIDs, SupportIDs [][]int64
	NativeFaces                             [][]int
}

// NonplanarSplineMaterial is an opaque authenticated material. Its source tag
// and detailed payload cannot be independently forged through exported fields.
type NonplanarSplineMaterial struct {
	kind         NonplanarSplineMaterialKind
	source, name string
	unit         float64
	solid        trimmed.Solid
	provenance   NonplanarSplineMaterialProvenance
	payload      any
}

func (m NonplanarSplineMaterial) Kind() NonplanarSplineMaterialKind { return m.kind }
func (m NonplanarSplineMaterial) OriginalSTEP() string              { return m.source }
func (m NonplanarSplineMaterial) Name() string                      { return m.name }
func (m NonplanarSplineMaterial) LengthUnit() float64               { return m.unit }
func (m NonplanarSplineMaterial) Solid() trimmed.Solid              { return m.solid }
func (m NonplanarSplineMaterial) Provenance() NonplanarSplineMaterialProvenance {
	p := m.provenance
	p.ShellIDs = append([]int64(nil), p.ShellIDs...)
	p.VoidOccurrenceIDs = append([]int64(nil), p.VoidOccurrenceIDs...)
	copyIDs := func(a [][]int64) [][]int64 {
		b := make([][]int64, len(a))
		for i, v := range a {
			b[i] = append([]int64(nil), v...)
		}
		return b
	}
	p.FaceIDs = copyIDs(p.FaceIDs)
	p.EdgeIDs = copyIDs(p.EdgeIDs)
	p.PCurveIDs = copyIDs(p.PCurveIDs)
	p.SupportIDs = copyIDs(p.SupportIDs)
	p.NativeFaces = make([][]int, len(m.provenance.NativeFaces))
	for i, v := range m.provenance.NativeFaces {
		p.NativeFaces[i] = append([]int(nil), v...)
	}
	return p
}

func nonplanarMaterialSignature(f *File) (NonplanarSplineMaterialKind, int64, error) {
	fail := func() (NonplanarSplineMaterialKind, int64, error) { return "", 0, geom.ErrInvalid }
	counts := map[string][]int64{}
	maxDegree := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, r := range en.Records {
			counts[r.Type] = append(counts[r.Type], id)
			if r.Type == "RATIONAL_B_SPLINE_SURFACE" {
				return fail()
			}
			if r.Type == "B_SPLINE_SURFACE_WITH_KNOTS" {
				if len(en.Records) != 1 || len(r.Args) != 13 {
					return fail()
				}
				for _, j := range []int{1, 2} {
					q, e := affineSplineSourceRat(r.Args[j])
					if e != nil || !q.IsInt() || q.Cmp(big.NewRat(1, 1)) < 0 || q.Cmp(big.NewRat(3, 1)) > 0 {
						return fail()
					}
					maxDegree = max(maxDegree, int(q.Num().Int64()))
				}
			}
		}
	}
	if maxDegree == 0 || len(counts["ADVANCED_BREP_SHAPE_REPRESENTATION"]) != 1 {
		return fail()
	}
	if len(counts["BREP_WITH_VOIDS"]) == 1 && len(counts["MANIFOLD_SOLID_BREP"]) == 0 {
		body := counts["BREP_WITH_VOIDS"][0]
		r, e := simple(f, body, "BREP_WITH_VOIDS", 3)
		if e != nil {
			return fail()
		}
		v, e := listRefs(r.Args[2], 1)
		if e != nil || len(v) < 1 || len(v) > 4 || len(counts["CLOSED_SHELL"]) != len(v)+1 || len(counts["ORIENTED_CLOSED_SHELL"]) != len(v) || maxDegree > 2 || len(counts["RATIONAL_B_SPLINE_CURVE"]) != 0 {
			return fail()
		}
		if len(v) == 1 {
			return NonplanarCavityMaterial, body, nil
		}
		return NonplanarMultipleVoidsMaterial, body, nil
	}
	if len(counts["MANIFOLD_SOLID_BREP"]) != 1 || len(counts["BREP_WITH_VOIDS"]) != 0 || len(counts["CLOSED_SHELL"]) != 1 || len(counts["ORIENTED_CLOSED_SHELL"]) != 0 {
		return fail()
	}
	body := counts["MANIFOLD_SOLID_BREP"][0]
	if len(counts["RATIONAL_B_SPLINE_CURVE"]) > 0 {
		if maxDegree > 2 {
			return fail()
		}
		return NonplanarRationalSpeedMaterial, body, nil
	}
	multi := false
	for _, id := range counts["PCURVE"] {
		p, e := simple(f, id, "PCURVE", 3)
		if e != nil || p.Args[2].Kind != Reference {
			return fail()
		}
		rep, e := simple(f, p.Args[2].Ref, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		ids, e := listRefs(rep.Args[1], 1)
		if e != nil || len(ids) != 1 {
			return fail()
		}
		c, e := simple(f, ids[0], "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if e != nil || c.Args[7].Kind != Aggregate {
			return fail()
		}
		d, e := affineSplineSourceRat(c.Args[1])
		if e != nil || !d.IsInt() || d.Cmp(big.NewRat(1, 1)) < 0 || d.Cmp(big.NewRat(8, 1)) > 0 {
			return fail()
		}
		multi = multi || d.Cmp(big.NewRat(1, 1)) > 0 || len(c.Args[7].Items) > 2
	}
	if multi {
		if maxDegree > 2 {
			return fail()
		}
		return NonplanarMultispanUVMaterial, body, nil
	}
	if maxDegree == 3 {
		return NonplanarCubicMaterial, body, nil
	}
	return NonplanarQuadraticMaterial, body, nil
}

// ReadNonplanarSplineMaterial classifies actual source topology/basis and invokes
// exactly one detailed strict validator. It never retries another family after
// rejection. Parsing and geometric proof retain the existing bounded limits.
func ReadNonplanarSplineMaterial(ctx context.Context, r io.Reader, l Limits) (NonplanarSplineMaterial, error) {
	var zero NonplanarSplineMaterial
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 2048)
	l.MaxValues = min(l.MaxValues, 32768)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	kind, bodyID, e := nonplanarMaterialSignature(f)
	if e != nil {
		return zero, e
	}
	m := NonplanarSplineMaterial{kind: kind, source: string(raw)}
	m.provenance.BodyID = bodyID
	single := func(model SplineShellModel) {
		m.name = model.Bodies[0].Name
		m.unit = model.LengthUnit
		m.solid = model.Bodies[0].Solid
		m.provenance.NativeFaces = [][]int{{0, 1, 2, 3, 4, 5}}
	}
	switch kind {
	case NonplanarQuadraticMaterial:
		q, err := ReadNonplanarSplineShellSource(ctx, bytes.NewReader(raw), l)
		if err != nil {
			return zero, err
		}
		m.payload = q
		single(q.Model)
	case NonplanarCubicMaterial:
		q, err := ReadCubicNonplanarSplineShellSource(ctx, bytes.NewReader(raw), l)
		if err != nil {
			return zero, err
		}
		m.payload = q
		single(q.Model)
	case NonplanarMultispanUVMaterial:
		q, err := ReadMultispanNonplanarSplineShellSource(ctx, bytes.NewReader(raw), l)
		if err != nil {
			return zero, err
		}
		m.payload = q
		single(q.Model)
	case NonplanarRationalSpeedMaterial:
		q, err := ReadRationalSpeedNonplanarSplineShellSource(ctx, bytes.NewReader(raw), l)
		if err != nil {
			return zero, err
		}
		m.payload = q
		single(q.Model)
	case NonplanarCavityMaterial:
		q, err := ReadNonplanarSplineCavitySource(ctx, bytes.NewReader(raw), l)
		if err != nil {
			return zero, err
		}
		m.payload = q
		m.name = q.Name
		m.unit = q.LengthUnit
		m.solid = q.Solid
		h := q.History()
		m.provenance.NativeFaces = [][]int{h.FacesA, h.FacesB}
	case NonplanarMultipleVoidsMaterial:
		q, err := ReadNonplanarSplineVoidsSource(ctx, bytes.NewReader(raw), l)
		if err != nil {
			return zero, err
		}
		m.payload = q
		m.name = q.Name
		m.unit = q.LengthUnit
		m.solid = q.Solid
		m.provenance.NativeFaces = q.NativeFaces()
	default:
		return zero, geom.ErrInvalid
	}
	b := f.Entities[bodyID].Records[0]
	m.provenance.ShellIDs = []int64{b.Args[1].Ref}
	if b.Type == "BREP_WITH_VOIDS" {
		for _, v := range b.Args[2].Items {
			m.provenance.VoidOccurrenceIDs = append(m.provenance.VoidOccurrenceIDs, v.Ref)
			o, _ := simple(f, v.Ref, "ORIENTED_CLOSED_SHELL", 4)
			m.provenance.ShellIDs = append(m.provenance.ShellIDs, o.Args[2].Ref)
		}
	}
	for _, sid := range m.provenance.ShellIDs {
		shell, _ := simple(f, sid, "CLOSED_SHELL", 2)
		faces, e := listRefs(shell.Args[1], 6)
		if e != nil {
			return zero, e
		}
		m.provenance.FaceIDs = append(m.provenance.FaceIDs, faces)
		var supports, edges, pcs []int64
		for _, fid := range faces {
			face, _ := simple(f, fid, "ADVANCED_FACE", 4)
			supports = append(supports, face.Args[2].Ref)
		}
		cl, e := nauoClosure(f, []int64{sid}, 32)
		if e != nil {
			return zero, e
		}
		for _, id := range f.Order {
			if !cl[id] {
				continue
			}
			if _, e := simple(f, id, "EDGE_CURVE", 5); e == nil {
				edges = append(edges, id)
			}
			if _, e := simple(f, id, "PCURVE", 3); e == nil {
				pcs = append(pcs, id)
			}
		}
		m.provenance.SupportIDs = append(m.provenance.SupportIDs, supports)
		m.provenance.EdgeIDs = append(m.provenance.EdgeIDs, edges)
		m.provenance.PCurveIDs = append(m.provenance.PCurveIDs, pcs)
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
