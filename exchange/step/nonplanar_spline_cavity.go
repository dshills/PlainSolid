package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// NonplanarSplineCavitySource retains two individually outward original spline
// shells and their proved material difference. Source ShellIDs are outer/inner;
// the original inner ORIENTED_CLOSED_SHELL occurrence must be reversed.
type NonplanarSplineCavitySource struct {
	Name               string
	LengthUnit         float64
	Outer, Inner       SplineShellBody
	Solid              trimmed.Solid
	original           string
	bodyID, orientedID int64
	shellIDs           [2]int64
	faces, edges       [2][]int64
	curves             [2][]SplineShellPCurveEvidence
	history            trimmed.SplineShellBooleanHistory
}

func (m NonplanarSplineCavitySource) SourceBody() int64           { return m.bodyID }
func (m NonplanarSplineCavitySource) SourceVoidOccurrence() int64 { return m.orientedID }
func (m NonplanarSplineCavitySource) PCurves() [2][]SplineShellPCurveEvidence {
	return [2][]SplineShellPCurveEvidence{append([]SplineShellPCurveEvidence(nil), m.curves[0]...), append([]SplineShellPCurveEvidence(nil), m.curves[1]...)}
}
func (m NonplanarSplineCavitySource) OriginalSTEP() string   { return m.original }
func (m NonplanarSplineCavitySource) SourceShells() [2]int64 { return m.shellIDs }
func (m NonplanarSplineCavitySource) SourceFaces() [2][]int64 {
	return [2][]int64{append([]int64(nil), m.faces[0]...), append([]int64(nil), m.faces[1]...)}
}
func (m NonplanarSplineCavitySource) SourceEdges() [2][]int64 {
	return [2][]int64{append([]int64(nil), m.edges[0]...), append([]int64(nil), m.edges[1]...)}
}
func (m NonplanarSplineCavitySource) History() trimmed.SplineShellBooleanHistory {
	h := m.history
	h.FacesA = append([]int(nil), h.FacesA...)
	h.FacesB = append([]int(nil), h.FacesB...)
	return h
}

// ReadNonplanarSplineCavitySource proves original BREP_WITH_VOIDS ownership,
// both complete nonplanar rectangular shell embeddings, strict separation and
// containment, and inward material orientation. Contact or unresolved kernel
// proofs fail without output. This is not an incidence-only solid promotion.
func ReadNonplanarSplineCavitySource(ctx context.Context, r io.Reader, l Limits) (NonplanarSplineCavitySource, error) {
	var zero NonplanarSplineCavitySource
	fail := func() (NonplanarSplineCavitySource, error) { return zero, geom.ErrInvalid }
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
	if l.MaxFaces < 12 || l.MaxEdgeUses < 48 {
		return fail()
	}
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
	points := 0
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			counts[rec.Type] = append(counts[rec.Type], id)
			if rec.Type == "CARTESIAN_POINT" {
				points++
			}
		}
	}
	if points > l.MaxVertices {
		return fail()
	}
	for kind, n := range map[string]int{"BREP_WITH_VOIDS": 1, "MANIFOLD_SOLID_BREP": 0, "ORIENTED_CLOSED_SHELL": 1, "CLOSED_SHELL": 2, "ADVANCED_FACE": 12, "EDGE_CURVE": 24, "ADVANCED_BREP_SHAPE_REPRESENTATION": 1} {
		if len(counts[kind]) != n {
			return fail()
		}
	}
	m := NonplanarSplineCavitySource{original: string(raw), bodyID: counts["BREP_WITH_VOIDS"][0], orientedID: counts["ORIENTED_CLOSED_SHELL"][0]}
	body, e := simple(f, m.bodyID, "BREP_WITH_VOIDS", 3)
	if e != nil {
		return fail()
	}
	m.Name, e = textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	m.shellIDs[0], e = ref(body.Args[1])
	if e != nil {
		return fail()
	}
	orient, e := simple(f, m.orientedID, "ORIENTED_CLOSED_SHELL", 4)
	if e != nil {
		return fail()
	}
	m.shellIDs[1], e = ref(orient.Args[2])
	if e != nil {
		return fail()
	}
	closures := [2]map[int64]bool{}
	for i, id := range m.shellIDs {
		closures[i], e = nauoClosure(f, []int64{id}, 32)
		if e != nil {
			return fail()
		}
	}
	for id := range closures[0] {
		if !closures[1][id] {
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
	if _, e = splitWrappedVoid(f, 6, false, WrappedSurfaceOptions{}); e != nil {
		return zero, e
	}
	var bodyIDs []int64
	for _, id := range f.Order {
		if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
			bodyIDs = append(bodyIDs, id)
		}
	}
	if len(bodyIDs) != 2 {
		return fail()
	}
	if bodyIDs[0] != m.bodyID {
		bodyIDs[0], bodyIDs[1] = bodyIDs[1], bodyIDs[0]
	}
	if bodyIDs[0] != m.bodyID {
		return fail()
	}
	for i, id := range bodyIDs {
		closures[i], e = nauoClosure(f, []int64{id}, 32)
		if e != nil {
			return fail()
		}
	}
	staged := serializeWrappedProofFile(f)
	var proofs [2]NonplanarSplineShellSource
	for i := 0; i < 2; i++ {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		sf, e := Parse(bytes.NewReader(staged), l)
		if e != nil {
			return zero, e
		}
		for id := range closures[1-i] {
			if !closures[i][id] {
				delete(sf.Entities, id)
			}
		}
		repID := counts["ADVANCED_BREP_SHAPE_REPRESENTATION"][0]
		rep := sf.Entities[repID].Records[0]
		var items []Value
		for _, v := range rep.Args[1].Items {
			if v.Kind == Reference && v.Ref == bodyIDs[1-i] {
				continue
			}
			items = append(items, v)
		}
		rep.Args[1] = Value{Kind: Aggregate, Items: items}
		en := sf.Entities[repID]
		en.Records[0] = rep
		sf.Entities[repID] = en
		view := serializeWrappedProofFile(sf)
		proofs[i], e = ReadNonplanarSplineShellSource(ctx, bytes.NewReader(view), l)
		if e != nil {
			return zero, fmt.Errorf("cavity shell %d: %w", i, e)
		}
		if len(proofs[i].Model.Bodies) != 1 {
			return fail()
		}
		for _, uv := range proofs[i].PCurves() {
			if uv.Knots != [2]string{"0", "1"} {
				return fail()
			}
		}
		for _, face := range proofs[i].Model.Bodies[0].Faces {
			if face.Domain != (geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}) {
				return fail()
			}
		}
		m.faces[i] = proofs[i].SourceFaces()
		m.edges[i] = proofs[i].SourceEdges()
		m.curves[i] = proofs[i].PCurves()
	}
	m.Outer = proofs[0].Model.Bodies[0]
	m.Inner = proofs[1].Model.Bodies[0]
	m.LengthUnit = proofs[0].Model.LengthUnit
	if proofs[1].Model.LengthUnit != m.LengthUnit {
		return fail()
	}
	m.Solid, m.history, e = trimmed.CombineSeparatedSplineShells(ctx, m.Outer.Solid, m.Inner.Solid, parametric.Difference, trimmed.SplineShellBooleanOptions{})
	if e != nil {
		return zero, e
	}
	if !m.history.BInsideA || m.history.AInsideB || !m.history.BReversed || len(m.Solid.Topology().Shells) != 2 {
		return fail()
	}
	return m, nil
}
