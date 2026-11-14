package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"strings"
)

// PlacedEllipticalPrismMaterial is an explicitly placed filled or annular
// material. Exactly one local topology pointer is nonnil. Original local model
// coefficients and world source vertices/terminals remain separate.
type PlacedEllipticalPrismMaterial struct {
	Filled                  *EllipticalPrismTopology
	Annular                 *EllipticalAnnularPrismTopology
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	SourceVertices          []geom.Vec3
	SourceTerminalPoints    []geom.Vec3
	Solid                   trimmed.Solid
	NativeFaceSourceIDs     []int
	NativeRimEdges          [2][2][4]int
	source                  []byte
}

func ellipsePlacedLeaf(kind int) bool { return kind == 104 || kind == 110 || kind == 190 }
func ReadPlacedEllipticalPrismMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (PlacedEllipticalPrismMaterial, error) {
	bad := func() (PlacedEllipticalPrismMaterial, error) {
		return PlacedEllipticalPrismMaterial{}, fmt.Errorf("iges: unsupported placed exact ellipse material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return PlacedEllipticalPrismMaterial{}, e
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
		return PlacedEllipticalPrismMaterial{}, e
	}
	n := len(doc.entities)
	if n != 21 && n != 29 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	source := append([]byte(nil), doc.bytes...)
	tr := doc.entities[n-1]
	if tr.kind != 124 || tr.id != 2*n-1 || tr.transform != 0 || len(tr.values) != 13 || compositeSourceNumbers(124, tr.values) != nil {
		return bad()
	}
	k, e := intValue(tr.values[0])
	if e != nil || k != 124 {
		return bad()
	}
	pose, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(pose) {
		return bad()
	}
	out := PlacedEllipticalPrismMaterial{Placement: pose, SourceTransformID: tr.id, SourceTransformMetadata: CylinderDatumMetadata{tr.label, tr.subscript}, source: source}
	for i, rec := range doc.entities[:n-1] {
		want := 0
		if ellipsePlacedLeaf(rec.kind) {
			want = tr.id
		}
		if rec.transform != want {
			return bad()
		}
		doc.entities[i].transform = 0
		if rec.kind != 502 && rec.kind != 122 {
			continue
		}
		if compositeSourceNumbers(110, rec.values) != nil {
			return bad()
		}
		count := 1
		start := 2
		if rec.kind == 502 {
			count, e = intValue(rec.values[1])
			if e != nil || (count != 2 && count != 4) || len(rec.values) != 2+count*3 {
				return bad()
			}
		} else if len(rec.values) != 5 {
			return bad()
		}
		v := append([]parameter(nil), rec.values...)
		for j := 0; j < count; j++ {
			p := [3]float64{}
			for axis := range p {
				p[axis], e = realValue(v[start+3*j+axis])
				if e != nil {
					return bad()
				}
			}
			world := geom.Vec3{X: p[0], Y: p[1], Z: p[2]}
			if rec.kind == 502 {
				out.SourceVertices = append(out.SourceVertices, world)
			} else {
				out.SourceTerminalPoints = append(out.SourceTerminalPoints, world)
			}
			local, er := roofMapPoint(pose, world, true)
			if er != nil {
				return bad()
			}
			for axis, x := range []float64{local.X, local.Y, local.Z} {
				v[start+3*j+axis].text = number(x)
			}
		}
		doc.entities[i].values = v
	}
	doc.entities = doc.entities[:n-1]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	var d EllipticalAnnularPrismDescriptor
	if n == 21 {
		m, er := ReadEllipticalPrismTopology(ctx, bytes.NewReader(raw), l)
		if er != nil {
			return bad()
		}
		out.Filled = &m
		d = EllipticalAnnularPrismDescriptor{OuterCoefficients: m.Descriptor.Coefficients, LowerZ: m.Descriptor.LowerZ, UpperZ: m.Descriptor.UpperZ}
		out.NativeFaceSourceIDs = []int{35, 33, 31, 31, 31, 31}
	} else {
		m, er := ReadEllipticalAnnularPrismTopology(ctx, bytes.NewReader(raw), l)
		if er != nil {
			return bad()
		}
		out.Annular = &m
		d = m.Descriptor
		out.NativeFaceSourceIDs = []int{51, 49, 45, 45, 45, 45, 47, 47, 47, 47}
	}
	origin, e := roofMapPoint(pose, geom.Vec3{Z: d.LowerZ}, false)
	if e != nil {
		return bad()
	}
	frame, e := geom.NewFrame(origin, pose.ApplyVector(geom.Vec3{X: 1}), pose.ApplyVector(geom.Vec3{Z: 1}))
	if e != nil {
		return bad()
	}
	a, b, e := ellipseConicRadii(EllipseConic{Coefficients: d.OuterCoefficients, Z: d.LowerZ})
	if e != nil {
		return bad()
	}
	height, e := cylinderExactSum(d.UpperZ, -d.LowerZ)
	if e != nil {
		return bad()
	}
	if n == 21 {
		var rim [2][4]int
		out.Solid, rim, e = ellipticprism.Prism(ctx, frame, a, b, height, tol)
		out.NativeRimEdges[0] = rim
	} else {
		ia, ib, er := ellipseConicRadii(EllipseConic{Coefficients: d.InnerCoefficients, Z: d.LowerZ})
		if er != nil {
			return bad()
		}
		out.Solid, out.NativeRimEdges, e = ellipticprism.Annulus(ctx, frame, a, b, ia, ib, height, tol)
	}
	if e != nil {
		return PlacedEllipticalPrismMaterial{}, e
	}
	return out, nil
}
func writePlacedEllipse(ctx context.Context, w io.Writer, a topologyAuthor, m geom.Affine, unit float64, tol float64) error {
	return writePlacedConic(ctx, w, a, m, unit, tol, func(raw []byte) error {
		_, e := ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, tol)
		return e
	})
}
func writePlacedConic(ctx context.Context, w io.Writer, a topologyAuthor, m geom.Affine, unit, tol float64, validate func([]byte) error) error {
	return writePlacedConicLeaves(ctx, w, a, m, unit, tol, ellipsePlacedLeaf, validate)
}
func writePlacedConicLeaves(ctx context.Context, w io.Writer, a topologyAuthor, m geom.Affine, unit, tol float64, leaf func(int) bool, validate func([]byte) error) error {
	if ctx == nil || w == nil || !roofRigid(m) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	for i, r := range a.records {
		if r.kind != 502 && r.kind != 122 {
			continue
		}
		budget := 2048
		v, e := parseParameters(r.data, ',', ';', &budget)
		if e != nil {
			return e
		}
		count := 1
		if r.kind == 502 {
			count, e = intValue(v[1])
			if e != nil {
				return e
			}
		}
		for j := 0; j < count; j++ {
			p := [3]float64{}
			for k := range p {
				p[k], e = realValue(v[2+3*j+k])
				if e != nil {
					return e
				}
			}
			q, e := roofMapPoint(m, geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, false)
			if e != nil {
				return e
			}
			for k, x := range []float64{q.X, q.Y, q.Z} {
				v[2+3*j+k].text = number(x)
			}
		}
		s := []string{}
		for _, p := range v {
			s = append(s, p.text)
		}
		a.records[i].data = strings.Join(s, ",") + ";"
	}
	nums := []string{"124"}
	for _, row := range m {
		for _, x := range row {
			nums = append(nums, number(x))
		}
	}
	tid := a.add(124, 0, strings.Join(nums, ",")+";")
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		id, _ := field(string(line[73:]))
		if id%2 != 1 {
			continue
		}
		kind, _ := field(string(line[:8]))
		if leaf(kind) {
			copy(line[48:56], fmt.Sprintf("%8d", tid))
		}
	}
	if e = validate(raw); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
func WritePlacedEllipticalPrismTopology(ctx context.Context, w io.Writer, d EllipticalPrismDescriptor, m geom.Affine, unit, tol float64) error {
	a, e := ellipticalPrismAuthor(d)
	if e != nil {
		return e
	}
	return writePlacedEllipse(ctx, w, a, m, unit, tol)
}
func WritePlacedEllipticalAnnularPrismTopology(ctx context.Context, w io.Writer, d EllipticalAnnularPrismDescriptor, m geom.Affine, unit, tol float64) error {
	a, e := ellipticalAnnularPrismAuthor(d)
	if e != nil {
		return e
	}
	return writePlacedEllipse(ctx, w, a, m, unit, tol)
}
