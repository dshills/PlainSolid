package iges

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
)

// EllipseDiskFace is one exact type144 planar disk, not a closed material.
// Model is the full model-space104 ellipse; UV uses identical coefficients at
// Z=0. The owning190 chart is (u,v)->(u,v,Model.Curve.Z), with positive normal.
type EllipseDiskFace struct {
	source                                                           []byte
	Model                                                            EllipseConicModel
	Metadata, PlaneMetadata, ParameterMetadata, BoundaryMetadata     CylinderDatumMetadata
	PointMetadata, NormalMetadata, ReferenceMetadata                 CylinderDatumMetadata
	SourceFaceID, SourcePlaneID, SourceBoundaryID, SourceParameterID int
	SourcePointID, SourceNormalID, SourceReferenceID                 int
}

// NativeFace reconstructs the exact analytic plane and full elliptical trim.
// The returned model ellipse uses radians; the native pcurve uses normalized
// t in[0,1], with model angle=2πt. This explicit parameter map is retained.
func (m EllipseDiskFace) NativeFace() (trimmed.Face, geom.Ellipse, error) {
	bad := func() (trimmed.Face, geom.Ellipse, error) { return trimmed.Face{}, geom.Ellipse{}, geom.ErrInvalid }
	if m.Model.Curve.Placement != nil || m.Model.Curve.PlacementMetadata != (CylinderDatumMetadata{}) || !finite(m.Model.LengthUnit) || m.Model.LengthUnit <= 0 {
		return bad()
	}
	for _, x := range []CylinderDatumMetadata{m.Metadata, m.PlaneMetadata, m.ParameterMetadata, m.BoundaryMetadata, m.PointMetadata, m.NormalMetadata, m.ReferenceMetadata} {
		if cylinderMetadata(x) != nil {
			return bad()
		}
	}
	c, e := m.Model.Curve.Geometry()
	if e != nil {
		return bad()
	}
	a, b, e := ellipseConicRadii(m.Model.Curve)
	if e != nil {
		return bad()
	}
	frame, e := geom.NewFrame(geom.Vec3{Z: m.Model.Curve.Z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return bad()
	}
	p, e := geom.NewPlaneSurface(frame)
	if e != nil {
		return bad()
	}
	uv := trimmed.EllipseUV{Cos: geom.Vec2{X: a}, Sin: geom.Vec2{Y: b}, To: 2 * math.Pi}
	f := trimmed.Face{Surface: p, Outer: []trimmed.Use{{Edge: 0, UVStart: geom.Vec2{X: a}, UVEnd: geom.Vec2{X: a}, Curve: uv}}}
	return f, c, nil
}

func ReadEllipseDiskFace(r io.Reader, l Limits) (EllipseDiskFace, error) {
	bad := func() (EllipseDiskFace, error) {
		return EllipseDiskFace{}, fmt.Errorf("iges: unsupported exact ellipse disk face")
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 256)
	l.MaxEntities = min(l.MaxEntities, 8)
	l.MaxValues = min(l.MaxValues, 1024)
	d, e := readPlacedTopologyRecords(r, l, false, false, false, false, false, false, false, false, false, true, true, true, true)
	if e != nil {
		return EllipseDiskFace{}, e
	}
	if len(d.entities) != 8 {
		return bad()
	}
	by := map[int]topologyRecord{}
	root := 0
	for _, x := range d.entities {
		if x.transform != 0 || compositeSourceNumbers(110, x.values) != nil {
			return bad()
		}
		k, er := intValue(x.values[0])
		if er != nil || k != x.kind {
			return bad()
		}
		by[x.id] = x
		if x.kind == 144 {
			if root != 0 {
				return bad()
			}
			root = x.id
		}
	}
	seen := map[int]bool{}
	get := func(id, kind, n int) (topologyRecord, bool) {
		x, ok := by[id]
		if !ok || x.kind != kind || len(x.values) != n || seen[id] {
			return x, false
		}
		seen[id] = true
		return x, true
	}
	integer := func(x topologyRecord, i int) int {
		v, er := intValue(x.values[i])
		if er != nil {
			return -1
		}
		return v
	}
	meta := func(x topologyRecord) CylinderDatumMetadata { return CylinderDatumMetadata{x.label, x.subscript} }
	face, ok := get(root, 144, 5)
	if !ok || integer(face, 2) != 1 || integer(face, 3) != 0 {
		return bad()
	}
	plane, ok := get(integer(face, 1), 190, 4)
	if !ok {
		return bad()
	}
	cos, ok := get(integer(face, 4), 142, 6)
	if !ok || integer(cos, 1) != 0 || integer(cos, 2) != plane.id || integer(cos, 5) != 3 {
		return bad()
	}
	uv, ok := get(integer(cos, 3), 104, 12)
	if !ok {
		return bad()
	}
	model, ok := get(integer(cos, 4), 104, 12)
	if !ok {
		return bad()
	}
	point, ok := get(integer(plane, 1), 116, 5)
	if !ok || integer(point, 4) != 0 {
		return bad()
	}
	normal, ok := get(integer(plane, 2), 123, 4)
	if !ok {
		return bad()
	}
	ref, ok := get(integer(plane, 3), 123, 4)
	if !ok || len(seen) != 8 {
		return bad()
	}
	mcurve, e := decodeEllipseConicRecords(topologySource{unit: d.unit, entities: []topologyRecord{model}}, nil)
	if e != nil {
		return bad()
	}
	pcurve, e := decodeEllipseConicRecords(topologySource{unit: d.unit, entities: []topologyRecord{uv}}, nil)
	if e != nil || pcurve.Curve.Z != 0 || pcurve.Curve.Coefficients != mcurve.Curve.Coefficients {
		return bad()
	}
	for _, want := range []struct {
		x topologyRecord
		v [3]float64
	}{{point, [3]float64{0, 0, mcurve.Curve.Z}}, {normal, [3]float64{0, 0, 1}}, {ref, [3]float64{1, 0, 0}}} {
		for i, v := range want.v {
			n, er := realValue(want.x.values[i+1])
			if er != nil || n != v {
				return bad()
			}
		}
	}
	for _, line := range bytes.Split(d.bytes, []byte("\n")) {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		id, er := field(string(line[73:]))
		if er != nil {
			return bad()
		}
		idx := []int{1, 2}
		if id%2 == 1 {
			idx = []int{3, 4}
			status, er := field(string(line[64:72]))
			want := 10000
			switch by[id].kind {
			case 144:
				want = 0
			case 116:
				want = 10400
			case 123:
				want = 10200
			case 142:
				want = 10500
			}
			if er != nil || status != want {
				return bad()
			}
		}
		for _, j := range idx {
			v, er := field(string(line[j*8 : j*8+8]))
			if er != nil || v != 0 {
				return bad()
			}
		}
	}
	out := EllipseDiskFace{source: append([]byte(nil), d.bytes...), Model: mcurve, Metadata: meta(face), PlaneMetadata: meta(plane), ParameterMetadata: meta(uv), BoundaryMetadata: meta(cos), PointMetadata: meta(point), NormalMetadata: meta(normal), ReferenceMetadata: meta(ref), SourceFaceID: face.id, SourcePlaneID: plane.id, SourceBoundaryID: cos.id, SourceParameterID: uv.id, SourcePointID: point.id, SourceNormalID: normal.id, SourceReferenceID: ref.id}
	if _, _, e = out.NativeFace(); e != nil {
		return bad()
	}
	return out, nil
}

func WriteEllipseDiskFace(w io.Writer, m EllipseDiskFace) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if _, _, e := m.NativeFace(); e != nil {
		return e
	}
	a := topologyAuthor{}
	a.add(144, 0, "144,3,1,0,11;")
	a.add(190, 1, "190,5,7,9;")
	a.add(116, 0, "116,0,0,"+number(m.Model.Curve.Z)+",0;")
	a.add(123, 0, "123,0,0,1;")
	a.add(123, 0, "123,1,0,0;")
	a.add(142, 0, "142,0,3,13,15,3;")
	major, _, e := ellipseConicRadii(m.Model.Curve)
	if e != nil {
		return e
	}
	for _, z := range []float64{0, m.Model.Curve.Z} {
		v := []string{"104"}
		for _, n := range m.Model.Curve.Coefficients {
			v = append(v, number(n))
		}
		v = append(v, number(z), number(major), "0", number(major), "0")
		a.add(104, 1, strings.Join(v, ",")+";")
	}
	metadata := []CylinderDatumMetadata{m.Metadata, m.PlaneMetadata, m.PointMetadata, m.NormalMetadata, m.ReferenceMetadata, m.BoundaryMetadata, m.ParameterMetadata, m.Model.Curve.Metadata}
	for i, x := range metadata {
		a.records[i].label = x.Label
		a.records[i].subscript = x.Subscript
	}
	raw, e := a.file(m.Model.LengthUnit)
	if e != nil {
		return e
	}
	status := []int{0, 10000, 10400, 10200, 10200, 10500, 10000, 10000}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		id, _ := field(string(line[73:]))
		if id%2 == 1 {
			copy(line[64:72], fmt.Sprintf("%08d", status[id/2]))
		}
	}
	if _, e = ReadEllipseDiskFace(bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
