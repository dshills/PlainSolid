package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"strings"
)

// CylinderSphereCavityMaterialDescriptor has a cylinder on the origin axis and
// an origin-centered sphere strictly between its caps and inside its radius.
type CylinderSphereCavityMaterialDescriptor struct {
	Stock        CircularCylinderDescriptor
	SphereRadius float64
}
type CylinderSphereCavityMaterial struct {
	Descriptor             CylinderSphereCavityMaterialDescriptor
	LengthUnit             float64
	Solid                  trimmed.Solid
	Parameters             map[int][]float64
	Metadata               map[int]CylinderDatumMetadata
	NativeRimEdges         [2][4]int
	NativeSphereEdges      [3]int
	NativeFaceSourceIDs    [7]int
	SourceSolidID          int
	SourceShellIDs         [2]int
	SourceFaceIDs          [4]int
	SourceVertexListIDs    [2]int
	SourceSphereMeridianID int
	SourceDegreeCurveIDs   [2][4]int
	source                 []byte
}

func cylinderSphereCavityMaterialAuthor(d CylinderSphereCavityMaterialDescriptor) (topologyAuthor, error) {
	a, e := circularCylinderAuthor(d.Stock)
	if e != nil {
		return a, e
	}
	inner, e := sphericalMaterialAuthor(SphericalMaterialDescriptor{Radius: d.SphereRadius})
	if e != nil {
		return a, e
	}
	top, e := cylinderExactSum(d.Stock.Origin.Z, d.Stock.Height)
	if e != nil {
		return a, e
	}
	if d.Stock.Origin.X != 0 || d.Stock.Origin.Y != 0 || d.Stock.Radius <= d.SphereRadius || d.Stock.Origin.Z >= -d.SphereRadius || top <= d.SphereRadius {
		return topologyAuthor{}, geom.ErrInvalid
	}
	for _, rec := range inner.records[1:] {
		budget := 2048
		v, e := parseParameters(rec.data, ',', ';', &budget)
		if e != nil {
			return topologyAuthor{}, e
		}
		slots, e := sphericalReferenceSlots(topologyRecord{kind: rec.kind, values: v})
		if e != nil {
			return topologyAuthor{}, e
		}
		for _, j := range slots {
			id, e := intValue(v[j])
			if e != nil {
				return topologyAuthor{}, e
			}
			v[j].text = fmt.Sprint(id + 46)
		}
		ss := []string{}
		for _, p := range v {
			ss = append(ss, p.text)
		}
		rec.data = strings.Join(ss, ",") + ";"
		a.records = append(a.records, rec)
	}
	a.records[23].data = "186,45,1,1,49,0;"
	return a, nil
}
func ReadCylinderSphereCavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (CylinderSphereCavityMaterial, error) {
	bad := func() (CylinderSphereCavityMaterial, error) {
		return CylinderSphereCavityMaterial{}, fmt.Errorf("iges: unsupported exact cylinder-sphere cavity material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return CylinderSphereCavityMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 39)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true, true)
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
			if circularCylinderIntegerField(rec.kind, j) || (rec.kind == 196 && (j == 1 || j == 3 || j == 4)) {
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
	o, t, wall, ball := values[1], values[3], values[9], values[53]
	if len(o) != 5 || len(t) != 5 || len(wall) != 5 || len(ball) != 5 {
		return bad()
	}
	height, e := cylinderExactSum(t[3], -o[3])
	if e != nil {
		return bad()
	}
	d := CylinderSphereCavityMaterialDescriptor{CircularCylinderDescriptor{Origin: geom.Vec3{X: o[1], Y: o[2], Z: o[3]}, Radius: wall[3], Height: height}, ball[2]}
	a, e := cylinderSphereCavityMaterialAuthor(d)
	if e != nil {
		return bad()
	}
	for i, rec := range doc.entities {
		w := a.records[i]
		tr := 0
		if rec.id == 69 {
			tr = 71
		}
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
	outer, rims, e := ellipticprism.CircularFrustum(ctx, d.Stock.Origin, d.Stock.Radius, d.Stock.Radius, d.Stock.Height, tol)
	if e != nil {
		return CylinderSphereCavityMaterial{}, e
	}
	ballNative, e := parametric.Sphere(geom.Vec3{}, d.SphereRadius)
	if e != nil {
		return bad()
	}
	inner, e := trimmed.FromParametric(ballNative)
	if e != nil {
		return bad()
	}
	v, edges, faces := outer.Vertices(), outer.Edges(), outer.Faces()
	nv, ne := len(v), len(edges)
	iv, ie, ifaces := inner.Vertices(), inner.Edges(), inner.Faces()
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
	solid, e := trimmed.NewSolid(append(v, iv...), append(edges, ie...), append(faces, ifaces...), tol)
	if e != nil {
		return CylinderSphereCavityMaterial{}, e
	}
	return CylinderSphereCavityMaterial{d, doc.unit, solid, values, metadata, rims, [3]int{ne, ne + 1, ne + 2}, [7]int{43, 41, 39, 39, 39, 39, 51}, 47, [2]int{45, 49}, [4]int{39, 41, 43, 51}, [2]int{15, 63}, 69, [2][4]int{{25, 27, 29, 31}, {65, 73, 75, 77}}, append([]byte(nil), doc.bytes...)}, nil
}
func WriteCylinderSphereCavityMaterial(ctx context.Context, w io.Writer, d CylinderSphereCavityMaterialDescriptor, unit, tol float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	a, e := cylinderSphereCavityMaterialAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			id, _ := field(string(line[73:]))
			if id == 69 {
				copy(line[48:56], "      71")
			}
		}
	}
	if _, e = ReadCylinderSphereCavityMaterial(ctx, bytes.NewReader(raw), Limits{}, tol); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
