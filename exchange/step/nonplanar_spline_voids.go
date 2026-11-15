package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// NonplanarSplineVoidsSource authenticates one outer spline shell and 2..4
// independently owned, strictly disjoint inward void shells. Shell/face order
// follows the original BREP_WITH_VOIDS list, not numeric entity ordering.
type NonplanarSplineVoidsSource struct {
	Name                  string
	LengthUnit            float64
	Outer                 SplineShellBody
	Voids                 []SplineShellBody
	Solid                 trimmed.Solid
	original              string
	bodyID                int64
	shellIDs, orientedIDs []int64
	faces, edges          [][]int64
	curves                [][]SplineShellPCurveEvidence
	nativeFaces           [][]int
}

func (m NonplanarSplineVoidsSource) OriginalSTEP() string { return m.original }
func (m NonplanarSplineVoidsSource) SourceBody() int64    { return m.bodyID }
func (m NonplanarSplineVoidsSource) SourceShells() []int64 {
	return append([]int64(nil), m.shellIDs...)
}
func (m NonplanarSplineVoidsSource) SourceVoidOccurrences() []int64 {
	return append([]int64(nil), m.orientedIDs...)
}
func (m NonplanarSplineVoidsSource) SourceFaces() [][]int64 {
	out := make([][]int64, len(m.faces))
	for i, p := range m.faces {
		out[i] = append([]int64(nil), p...)
	}
	return out
}
func (m NonplanarSplineVoidsSource) SourceEdges() [][]int64 {
	out := make([][]int64, len(m.edges))
	for i, p := range m.edges {
		out[i] = append([]int64(nil), p...)
	}
	return out
}
func (m NonplanarSplineVoidsSource) NativeFaces() [][]int {
	out := make([][]int, len(m.nativeFaces))
	for i, p := range m.nativeFaces {
		out[i] = append([]int(nil), p...)
	}
	return out
}
func (m NonplanarSplineVoidsSource) PCurves() [][]SplineShellPCurveEvidence {
	out := make([][]SplineShellPCurveEvidence, len(m.curves))
	for i, p := range m.curves {
		out[i] = append([]SplineShellPCurveEvidence(nil), p...)
	}
	return out
}

func ReadNonplanarSplineVoidsSource(ctx context.Context, r io.Reader, l Limits) (NonplanarSplineVoidsSource, error) {
	var zero NonplanarSplineVoidsSource
	fail := func() (NonplanarSplineVoidsSource, error) { return zero, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return fail()
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
		return fail()
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	counts := map[string][]int64{}
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			counts[rec.Type] = append(counts[rec.Type], id)
		}
	}
	if len(counts["BREP_WITH_VOIDS"]) != 1 {
		return fail()
	}
	m := NonplanarSplineVoidsSource{original: string(raw), bodyID: counts["BREP_WITH_VOIDS"][0]}
	body, e := simple(f, m.bodyID, "BREP_WITH_VOIDS", 3)
	if e != nil {
		return fail()
	}
	m.Name, e = textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	outer, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	m.orientedIDs, e = listRefs(body.Args[2], 2)
	if e != nil || len(m.orientedIDs) < 2 || len(m.orientedIDs) > 4 {
		return fail()
	}
	n := len(m.orientedIDs) + 1
	for kind, want := range map[string]int{"MANIFOLD_SOLID_BREP": 0, "ORIENTED_CLOSED_SHELL": n - 1, "CLOSED_SHELL": n, "ADVANCED_FACE": 6 * n, "EDGE_CURVE": 12 * n, "ADVANCED_BREP_SHAPE_REPRESENTATION": 1} {
		if len(counts[kind]) != want {
			return fail()
		}
	}
	if l.MaxFaces < 6*n || l.MaxEdgeUses < 24*n || len(counts["CARTESIAN_POINT"]) > l.MaxVertices {
		return fail()
	}
	m.shellIDs = []int64{outer}
	used := map[int64]bool{outer: true}
	orientUsed := map[int64]bool{}
	for _, id := range m.orientedIDs {
		if orientUsed[id] {
			return fail()
		}
		orientUsed[id] = true
		rec, e := simple(f, id, "ORIENTED_CLOSED_SHELL", 4)
		if e != nil || rec.Args[1].Kind != Derived {
			return fail()
		}
		sense, e := boolean(rec.Args[3])
		if e != nil || sense {
			return fail()
		}
		sid, e := ref(rec.Args[2])
		if e != nil || used[sid] {
			return fail()
		}
		used[sid] = true
		m.shellIDs = append(m.shellIDs, sid)
	}
	closures := make([]map[int64]bool, n)
	for i, id := range m.shellIDs {
		closures[i], e = nauoClosure(f, []int64{id}, 32)
		if e != nil {
			return fail()
		}
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for id := range closures[i] {
				if !closures[j][id] {
					continue
				}
				for _, rec := range f.Entities[id].Records {
					switch rec.Type {
					case "GEOMETRIC_REPRESENTATION_CONTEXT", "PARAMETRIC_REPRESENTATION_CONTEXT", "REPRESENTATION_CONTEXT":
					default:
						return fail()
					}
				}
			}
		}
	}
	m.faces = make([][]int64, n)
	m.edges = make([][]int64, n)
	m.curves = make([][]SplineShellPCurveEvidence, n)
	for i := 1; i < n; i++ {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		sf, e := Parse(bytes.NewReader(raw), l)
		if e != nil {
			return zero, e
		}
		for j := 1; j < n; j++ {
			if j == i {
				continue
			}
			delete(sf.Entities, m.orientedIDs[j-1])
			for id := range closures[j] {
				if !closures[0][id] && !closures[i][id] {
					delete(sf.Entities, id)
				}
			}
		}
		en := sf.Entities[m.bodyID]
		en.Records[0].Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: m.orientedIDs[i-1]}}}
		sf.Entities[m.bodyID] = en
		part, e := ReadNonplanarSplineCavitySource(ctx, bytes.NewReader(serializeWrappedProofFile(sf)), l)
		if e != nil {
			return zero, e
		}
		if i == 1 {
			m.Outer = part.Outer
			m.LengthUnit = part.LengthUnit
			m.faces[0] = part.faces[0]
			m.edges[0] = part.edges[0]
			m.curves[0] = part.curves[0]
		}
		if part.LengthUnit != m.LengthUnit {
			return fail()
		}
		m.Voids = append(m.Voids, part.Inner)
		m.faces[i] = part.faces[1]
		m.edges[i] = part.edges[1]
		m.curves[i] = part.curves[1]
	}
	parts := []trimmed.PlacedSolid{{Solid: m.Outer.Solid, Transform: geom.Identity()}}
	m.nativeFaces = make([][]int, n)
	for i := 0; i < n; i++ {
		for j := 0; j < 6; j++ {
			m.nativeFaces[i] = append(m.nativeFaces[i], 6*i+j)
		}
	}
	for _, b := range m.Voids {
		faces := b.Solid.Faces()
		for j := range faces {
			faces[j].Reversed = !faces[j].Reversed
		}
		inward, e := trimmed.NewSolidContext(ctx, b.Solid.Vertices(), b.Solid.Edges(), faces, b.Solid.Tolerance())
		if e != nil {
			return zero, e
		}
		parts = append(parts, trimmed.PlacedSolid{Solid: inward, Transform: geom.Identity()})
	}
	m.Solid, e = trimmed.AssembleShells(ctx, parts, 6*n)
	if e != nil {
		return zero, e
	}
	proof, e := m.Solid.ValidateNestedSplineComponents(ctx, trimmed.SplineShellBooleanOptions{})
	if e != nil {
		return zero, e
	}
	if len(proof) != n || proof[0].Parent != -1 || proof[0].Depth != 0 {
		return fail()
	}
	for i := 1; i < n; i++ {
		if proof[i].Parent != 0 || proof[i].Depth != 1 {
			return fail()
		}
	}
	return m, nil
}
