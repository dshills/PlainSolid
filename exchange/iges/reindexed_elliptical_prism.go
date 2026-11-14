package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
)

// ReindexedEllipticalPrismMaterial retains original record IDs separately from
// the canonical proof graph. Exactly one material pointer is nonnil. Nested
// material source IDs refer to the proof graph; SourceForCanonical maps them.
type ReindexedEllipticalPrismMaterial struct {
	Kind                                            string
	SourceForCanonical                              map[int]int
	SourceSolidID, SourceShellID, SourceTransformID int
	NativeFaceSourceIDs                             []int
	SourceRimCurveIDs                               [2][2]int
	Filled                                          *EllipticalPrismMaterial
	Annular                                         *EllipticalAnnularPrismMaterial
	Placed                                          *PlacedEllipticalPrismMaterial
	source                                          []byte
}

func (m ReindexedEllipticalPrismMaterial) Solid() trimmed.Solid {
	if m.Filled != nil {
		return m.Filled.Solid
	}
	if m.Annular != nil {
		return m.Annular.Solid
	}
	if m.Placed != nil {
		return m.Placed.Solid
	}
	return trimmed.Solid{}
}
func ellipseReferenceSlots(r topologyRecord) ([]int, error) {
	switch r.kind {
	case 104:
		if len(r.values) != 12 {
			return nil, geom.ErrInvalid
		}
		return nil, nil
	case 122:
		if len(r.values) != 5 {
			return nil, geom.ErrInvalid
		}
		if _, e := intValue(r.values[1]); e != nil {
			return nil, e
		}
		return []int{1}, nil
	}
	return topologyReferenceSlots(r)
}

// ReadReindexedEllipticalPrismMaterial accepts semantic record permutations of
// the four bounded filled/hollow/local/placed dialects. It preserves geometric
// tokens, proves a bijection of owned records, and invokes unchanged exact
// material readers. It does not reorder face, coedge or edge-list entries.
func ReadReindexedEllipticalPrismMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedEllipticalPrismMaterial, error) {
	bad := func() (ReindexedEllipticalPrismMaterial, error) {
		return ReindexedEllipticalPrismMaterial{}, fmt.Errorf("iges: unsupported reindexed ellipse material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ReindexedEllipticalPrismMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 29)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, false, false, true, true, true, true, true)
	if e != nil {
		return ReindexedEllipticalPrismMaterial{}, e
	}
	if mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	n := len(doc.entities)
	annular, placed := false, false
	switch n {
	case 20:
	case 21:
		placed = true
	case 28:
		annular = true
	case 29:
		annular, placed = true, true
	default:
		return bad()
	}
	d := EllipticalPrismDescriptor{Coefficients: [6]float64{4, 0, 25, 0, 0, -100}, LowerZ: 3, UpperZ: 7}
	a, e := ellipticalPrismAuthor(d)
	if annular {
		a, e = ellipticalAnnularPrismAuthor(EllipticalAnnularPrismDescriptor{OuterCoefficients: d.Coefficients, InnerCoefficients: [6]float64{1, 0, 4, 0, 0, -4}, LowerZ: 3, UpperZ: 7})
	}
	if e != nil {
		return bad()
	}
	tid := 0
	if placed {
		tid = a.add(124, 0, "124,1,0,0,0,0,1,0,0,0,0,1,0;")
	}
	want := map[int]topologyRecord{}
	root := 0
	for i, x := range a.records {
		budget := 2048
		v, er := parseParameters(x.data, ',', ';', &budget)
		if er != nil {
			return bad()
		}
		id := 2*i + 1
		tr := 0
		if placed && ellipsePlacedLeaf(x.kind) {
			tr = tid
		}
		want[id] = topologyRecord{id: id, kind: x.kind, values: v, transform: tr}
		if x.kind == 186 {
			root = id
		}
	}
	actual := map[int]topologyRecord{}
	sourceRoot := 0
	for _, x := range doc.entities {
		actual[x.id] = x
		if x.kind == 186 {
			if sourceRoot != 0 {
				return bad()
			}
			sourceRoot = x.id
		}
	}
	if sourceRoot == 0 {
		return bad()
	}
	forward, reverse := map[int]int{}, map[int]int{}
	queue := [][2]int{{root, sourceRoot}}
	steps := 0
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return ReindexedEllipticalPrismMaterial{}, e
		}
		steps++
		if steps > 512 {
			return bad()
		}
		pair := queue[0]
		queue = queue[1:]
		c, s := pair[0], pair[1]
		if old, ok := forward[c]; ok {
			if old != s {
				return bad()
			}
			continue
		}
		if _, ok := reverse[s]; ok {
			return bad()
		}
		w, ok := want[c]
		x, present := actual[s]
		if !ok || !present || w.kind != x.kind || len(w.values) != len(x.values) {
			return bad()
		}
		forward[c], reverse[s] = s, c
		wr, er := ellipseReferenceSlots(w)
		xr, ex := ellipseReferenceSlots(x)
		if er != nil || ex != nil || len(wr) != len(xr) {
			return bad()
		}
		for i, index := range wr {
			if index != xr[i] {
				return bad()
			}
			cw, er := intValue(w.values[index])
			sx, ex := intValue(x.values[index])
			if er != nil || ex != nil {
				return bad()
			}
			queue = append(queue, [2]int{cw, sx})
		}
		if w.transform == 0 {
			if x.transform != 0 {
				return bad()
			}
		} else {
			if x.transform == 0 {
				return bad()
			}
			queue = append(queue, [2]int{w.transform, x.transform})
		}
	}
	if len(forward) != n || len(reverse) != n {
		return bad()
	}
	for i := range a.records {
		id := 2*i + 1
		x := actual[forward[id]]
		slots, er := ellipseReferenceSlots(x)
		if er != nil {
			return bad()
		}
		v := append([]parameter(nil), x.values...)
		for _, j := range slots {
			target, er := intValue(v[j])
			if er != nil {
				return bad()
			}
			canonical, ok := reverse[target]
			if !ok {
				return bad()
			}
			v[j].text = fmt.Sprint(canonical)
		}
		str := []string{}
		for _, p := range v {
			if p.stringValue {
				return bad()
			}
			str = append(str, p.text)
		}
		a.records[i].data = strings.Join(str, ",") + ";"
		a.records[i].label = x.label
		a.records[i].subscript = x.subscript
	}
	raw, e := a.file(doc.unit)
	if e != nil {
		return bad()
	}
	if placed {
		for _, line := range bytes.Split(raw, []byte("\n")) {
			if len(line) != 80 || line[72] != 'D' {
				continue
			}
			id, _ := field(string(line[73:]))
			if id%2 != 1 {
				continue
			}
			copy(line[48:56], fmt.Sprintf("%8d", want[id].transform))
		}
	}
	out := ReindexedEllipticalPrismMaterial{SourceForCanonical: forward, SourceSolidID: sourceRoot, source: append([]byte(nil), doc.bytes...)}
	var faceIDs []int
	var rims [2][2]int
	if placed {
		m, er := ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader(raw), l, tol)
		if er != nil {
			return bad()
		}
		out.Placed = &m
		faceIDs = m.NativeFaceSourceIDs
		out.SourceTransformID = forward[tid]
		if annular {
			out.Kind = "placed-annular"
			out.SourceShellID = forward[53]
			rims = [2][2]int{{23, 19}, {29, 25}}
		} else {
			out.Kind = "placed-filled"
			out.SourceShellID = forward[37]
			rims = [2][2]int{{9, 11}, {0, 0}}
		}
	} else if annular {
		m, er := ReadEllipticalAnnularPrismMaterial(ctx, bytes.NewReader(raw), l, tol)
		if er != nil {
			return bad()
		}
		out.Annular = &m
		out.Kind = "annular"
		out.SourceShellID = forward[53]
		faceIDs = append(faceIDs, m.NativeFaceSourceIDs[:]...)
		rims = m.SourceRimCurveIDs
	} else {
		m, er := ReadEllipticalPrismMaterial(ctx, bytes.NewReader(raw), l, tol)
		if er != nil {
			return bad()
		}
		out.Filled = &m
		out.Kind = "filled"
		out.SourceShellID = forward[37]
		faceIDs = append(faceIDs, m.NativeFaceSourceIDs[:]...)
		rims[0] = m.SourceRimCurveIDs
	}
	for _, id := range faceIDs {
		out.NativeFaceSourceIDs = append(out.NativeFaceSourceIDs, forward[id])
	}
	for i := range rims {
		for j, id := range rims[i] {
			if id != 0 {
				out.SourceRimCurveIDs[i][j] = forward[id]
			}
		}
	}
	return out, nil
}

// WriteReindexedEllipticalPrismSource re-proves the retained original graph and
// writes its original record order/IDs. Edited geometry must use authoring APIs.
func WriteReindexedEllipticalPrismSource(ctx context.Context, w io.Writer, m ReindexedEllipticalPrismMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedEllipseEqual(ctx, m, p) {
		return fmt.Errorf("iges: stale reindexed ellipse material")
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}
