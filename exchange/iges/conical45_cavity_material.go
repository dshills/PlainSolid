package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"strings"
)

// Conical45CavityMaterialDescriptor contains strictly nested coaxial expanding
// slope-one frusta. The inner cap stations must be strictly interior.
type Conical45CavityMaterialDescriptor struct{ Outer, Inner Conical45FrustumDescriptor }
type Conical45CavityMaterial struct {
	Descriptor                          Conical45CavityMaterialDescriptor
	LengthUnit                          float64
	Solid                               trimmed.Solid
	Parameters                          map[int][]float64
	Metadata                            map[int]CylinderDatumMetadata
	NativeRimEdges                      [2][2][4]int
	NativeFaceSourceIDs                 [12]int
	SourceSolidID                       int
	SourceShellIDs, SourceVertexListIDs [2]int
	SourceFaceIDs                       [2][3]int
	SourceRimCurveIDs                   [2][2]int
	source                              []byte
}

func conical45CavityMaterialAuthor(d Conical45CavityMaterialDescriptor) (topologyAuthor, error) {
	a, e := conical45FrustumAuthor(d.Outer)
	if e != nil {
		return a, e
	}
	b, e := conical45FrustumAuthor(d.Inner)
	if e != nil {
		return a, e
	}
	if d.Outer.Origin.X != d.Inner.Origin.X || d.Outer.Origin.Y != d.Inner.Origin.Y {
		return topologyAuthor{}, geom.ErrInvalid
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	dz := new(big.Rat).Sub(rat(d.Inner.Origin.Z), rat(d.Outer.Origin.Z))
	innerTop := new(big.Rat).Add(dz, rat(d.Inner.Height))
	clearance := new(big.Rat).Sub(new(big.Rat).Add(rat(d.Outer.Radius), dz), rat(d.Inner.Radius))
	if dz.Sign() <= 0 || innerTop.Cmp(rat(d.Outer.Height)) >= 0 || clearance.Sign() <= 0 {
		return topologyAuthor{}, geom.ErrInvalid
	}
	for _, rec := range b.records[:19] {
		budget := 2048
		v, e := parseParameters(rec.data, ',', ';', &budget)
		if e != nil {
			return topologyAuthor{}, e
		}
		slots, e := conical45ReferenceSlots(topologyRecord{kind: rec.kind, values: v})
		if e != nil {
			return topologyAuthor{}, e
		}
		for _, j := range slots {
			id, e := intValue(v[j])
			if e != nil {
				return topologyAuthor{}, e
			}
			v[j].text = fmt.Sprint(id + 40)
		}
		ss := []string{}
		for _, p := range v {
			ss = append(ss, p.text)
		}
		rec.data = strings.Join(ss, ",") + ";"
		a.records = append(a.records, rec)
	}
	a.records[19].data = "186,37,1,1,77,0;"
	return a, nil
}
func ReadConical45CavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (Conical45CavityMaterial, error) {
	bad := func() (Conical45CavityMaterial, error) {
		return Conical45CavityMaterial{}, fmt.Errorf("iges: unsupported exact spherical cavity material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return Conical45CavityMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 39)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true, true)
	if e != nil || len(doc.entities) != 39 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	values := map[int][]float64{}
	metadata := map[int]CylinderDatumMetadata{}
	for _, rec := range doc.entities {
		if compositeSourceNumbers(110, rec.values) != nil {
			return bad()
		}
		v := make([]float64, len(rec.values))
		for j, p := range rec.values {
			if circularCylinderIntegerField(rec.kind, j) || (rec.kind == 194 && (j == 1 || j == 2 || j == 5 || j == 6 || j == 7)) {
				if _, e := intValue(p); e != nil {
					return bad()
				}
			}
			v[j], e = realValue(p)
			if e != nil {
				return bad()
			}
		}
		values[rec.id] = v
		metadata[rec.id] = CylinderDatumMetadata{rec.label, rec.subscript}
	}
	var d Conical45CavityMaterialDescriptor
	for i, offset := range []int{0, 40} {
		o, t, wall := values[1+offset], values[3+offset], values[9+offset]
		if len(o) != 5 || len(t) != 5 || len(wall) != 8 {
			return bad()
		}
		h, e := cylinderExactSum(t[3], -o[3])
		if e != nil {
			return bad()
		}
		part := Conical45FrustumDescriptor{Origin: geom.Vec3{X: o[1], Y: o[2], Z: o[3]}, Radius: wall[3], Height: h}
		if i == 0 {
			d.Outer = part
		} else {
			d.Inner = part
		}
	}
	a, e := conical45CavityMaterialAuthor(d)
	if e != nil {
		return bad()
	}
	for i, rec := range doc.entities {
		w := a.records[i]
		tr := 0
		if rec.id != 2*i+1 || rec.kind != w.kind || rec.transform != tr {
			return bad()
		}
		budget := 2048
		v, e := parseParameters(w.data, ',', ';', &budget)
		if e != nil || len(v) != len(rec.values) {
			return bad()
		}
		for j, p := range v {
			x, e := realValue(p)
			if e != nil || x != values[rec.id][j] {
				return bad()
			}
		}
	}
	var solids [2]trimmed.Solid
	var rims [2][2][4]int
	for i, part := range []Conical45FrustumDescriptor{d.Outer, d.Inner} {
		top, e := cylinderExactSum(part.Radius, part.Height)
		if e != nil {
			return bad()
		}
		solids[i], rims[i], e = ellipticprism.CircularFrustum(ctx, part.Origin, part.Radius, top, part.Height, tol)
		if e != nil {
			return Conical45CavityMaterial{}, e
		}
	}
	vertices, edges, faces := solids[0].Vertices(), solids[0].Edges(), solids[0].Faces()
	nv, ne := len(vertices), len(edges)
	iv, ie, ifaces := solids[1].Vertices(), solids[1].Edges(), solids[1].Faces()
	for i := range ie {
		ie[i].Start += nv
		ie[i].End += nv
	}
	for i := range ifaces {
		ifaces[i].Reversed = !ifaces[i].Reversed
		for j := range ifaces[i].Outer {
			ifaces[i].Outer[j].Edge += ne
		}
	}
	for k := range rims[1] {
		for j := range rims[1][k] {
			rims[1][k][j] += ne
		}
	}
	s, e := trimmed.NewSolid(append(vertices, iv...), append(edges, ie...), append(faces, ifaces...), tol)
	if e != nil {
		return Conical45CavityMaterial{}, e
	}
	return Conical45CavityMaterial{d, doc.unit, s, values, metadata, rims, [12]int{35, 33, 31, 31, 31, 31, 75, 73, 71, 71, 71, 71}, 39, [2]int{37, 77}, [2]int{15, 55}, [2][3]int{{31, 33, 35}, {71, 73, 75}}, [2][2]int{{19, 17}, {59, 57}}, append([]byte(nil), doc.bytes...)}, nil
}
func WriteConical45CavityMaterial(ctx context.Context, w io.Writer, d Conical45CavityMaterialDescriptor, unit, tol float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	a, e := conical45CavityMaterialAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadConical45CavityMaterial(ctx, bytes.NewReader(raw), Limits{}, tol); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
