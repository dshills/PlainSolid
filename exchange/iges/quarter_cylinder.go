package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
)

// QuarterCylinderDescriptor is a positive global X-to-Y quarter sector along +Z.
// Radius and height are positive; translation and all derived vertices are lossless.
type QuarterCylinderDescriptor struct {
	Origin         geom.Vec3
	Radius, Height float64 // positive radius and axial height
}

// QuarterCylinderSolid retains exact source parameters and original face ancestry.
// Its native rational charts use tan(angle/2), not the source cylinder angle.
type QuarterCylinderSolid struct {
	Descriptor                                                         QuarterCylinderDescriptor
	LengthUnit                                                         float64
	Solid                                                              parametric.Solid
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	SourceFaceIDs, SourceSurfaceIDs                                    [5]int
	NativeFaceSourceIDs                                                [5]int // wall, lower cap, upper cap, X radial, Y radial
	Parameters                                                         map[int][]float64
	Metadata                                                           map[int]CylinderDatumMetadata
}

func quarterCylinderFailure() error {
	return fmt.Errorf("iges: unsupported exact canonical cardinal quarter-cylinder material")
}
func quarterCylinderAuthor(d QuarterCylinderDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	if !d.Origin.IsFinite() || !finite(d.Radius) || !finite(d.Height) || d.Radius <= 0 || d.Height <= 0 {
		return a, quarterCylinderFailure()
	}
	top, e := cylinderExactSum(d.Origin.Z, d.Height)
	if e != nil {
		return a, e
	}
	xr, e := cylinderExactSum(d.Origin.X, d.Radius)
	if e != nil {
		return a, e
	}
	yr, e := cylinderExactSum(d.Origin.Y, d.Radius)
	if e != nil {
		return a, e
	}
	x, y, z := d.Origin.X, d.Origin.Y, d.Origin.Z
	add := func(k, f int, v ...float64) int {
		q := []string{fmt.Sprint(k)}
		for _, n := range v {
			q = append(q, strings.TrimSuffix(number(n), "."))
		}
		return a.add(k, f, strings.Join(q, ",")+";")
	}
	o := add(116, 0, x, y, z, 0)
	t := add(116, 0, x, y, top, 0)
	az := add(123, 0, 0, 0, 1)
	ax := add(123, 0, 1, 0, 0)
	ay := add(123, 0, 0, 1, 0)
	ny := add(123, 0, 0, -1, 0)
	nx := add(123, 0, -1, 0, 0)
	surfaces := []int{add(192, 1, float64(o), float64(az), d.Radius, float64(ax)), add(190, 1, float64(t), float64(az), float64(ax)), add(190, 1, float64(o), float64(az), float64(ax)), add(190, 1, float64(o), float64(ny), float64(ax)), add(190, 1, float64(o), float64(nx), float64(ay))}
	pts := [][3]float64{{x, y, z}, {xr, y, z}, {x, yr, z}, {x, y, top}, {xr, y, top}, {x, yr, top}}
	vv := []float64{6}
	for _, p := range pts {
		vv = append(vv, p[:]...)
	}
	vertices := add(502, 1, vv...)
	pairs := [][2]int{{0, 1}, {1, 2}, {2, 0}, {3, 4}, {4, 5}, {5, 3}, {0, 3}, {1, 4}, {2, 5}}
	curves := []int{}
	for i, p := range pairs {
		if i == 1 || i == 4 {
			curves = append(curves, add(100, 0, pts[p[0]][2], x, y, xr, y, x, yr))
		} else {
			v := append(append([]float64{}, pts[p[0]][:]...), pts[p[1]][:]...)
			curves = append(curves, add(110, 0, v...))
		}
	}
	ev := []float64{9}
	for i, p := range pairs {
		ev = append(ev, float64(curves[i]), float64(vertices), float64(p[0]+1), float64(vertices), float64(p[1]+1))
	}
	edges := add(504, 1, ev...)
	// Signed indices encode each directed model edge around the face.
	loops := []int{}
	for _, wire := range [][]int{{2, 9, -5, -8}, {4, 5, 6}, {1, 2, 3}, {1, 8, -4, -7}, {3, 7, -6, -9}} {
		v := []float64{float64(len(wire))}
		for _, n := range wire {
			sn := 1.
			if n < 0 {
				sn = 0
				n = -n
			}
			v = append(v, 0, float64(edges), float64(n), sn, 0)
		}
		loops = append(loops, add(508, 1, v...))
	}
	faces := []int{}
	for i, s := range surfaces {
		faces = append(faces, add(510, 1, float64(s), 1, 1, float64(loops[i])))
	}
	sh := []float64{5}
	for i, f := range faces {
		sn := 1.
		if i == 2 {
			sn = 0
		}
		sh = append(sh, float64(f), sn)
	}
	shell := add(514, 1, sh...)
	add(186, 0, float64(shell), 1, 0)
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	return a, nil
}

// ReadQuarterCylinderSolid accepts only the registered 35-entity model-only
// quarter-sector graph, proving every support, endpoint, incidence and sense.
// Alternate/vendor pcurve encodings and transforms are rejected atomically.
func ReadQuarterCylinderSolid(ctx context.Context, r io.Reader, l Limits) (QuarterCylinderSolid, error) {
	fail := func() (QuarterCylinderSolid, error) { return QuarterCylinderSolid{}, quarterCylinderFailure() }
	if ctx == nil || r == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return QuarterCylinderSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 35)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true)
	if e != nil {
		return QuarterCylinderSolid{}, e
	}
	if len(doc.entities) != 35 {
		return fail()
	}
	if e = mixedTopologyDirectory(doc.bytes); e != nil {
		return fail()
	}
	values := map[int][]float64{}
	metadata := map[int]CylinderDatumMetadata{}
	for _, rec := range doc.entities {
		if e = compositeSourceNumbers(110, rec.values); e != nil {
			return fail()
		}
		v := make([]float64, len(rec.values))
		for i, p := range rec.values {
			if circularCylinderIntegerField(rec.kind, i) {
				if _, err := intValue(p); err != nil {
					return fail()
				}
			}
			v[i], e = realValue(p)
			if e != nil {
				return fail()
			}
		}
		values[rec.id] = v
		metadata[rec.id] = CylinderDatumMetadata{Label: rec.label, Subscript: rec.subscript}
	}
	if len(values[1]) != 5 || len(values[3]) != 5 || len(values[15]) != 5 {
		return fail()
	}
	height, e := cylinderExactSum(values[3][3], -values[1][3])
	if e != nil {
		return fail()
	}
	d := QuarterCylinderDescriptor{Origin: geom.Vec3{X: values[1][1], Y: values[1][2], Z: values[1][3]}, Radius: values[15][3], Height: height}
	a, e := quarterCylinderAuthor(d)
	if e != nil {
		return fail()
	}
	for i, rec := range doc.entities {
		want := a.records[i]
		if rec.id != 2*i+1 || rec.kind != want.kind || rec.transform != 0 {
			return fail()
		}
		budget := 2048
		v, e := parseParameters(want.data, ',', ';', &budget)
		if e != nil || len(v) != len(rec.values) {
			return fail()
		}
		for j, p := range v {
			x, e := realValue(p)
			if e != nil || x != values[rec.id][j] {
				return fail()
			}
		}
	}
	frame, e := geom.NewFrame(d.Origin, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail()
	}
	solid, e := parametric.CardinalQuarterCylinder(frame, d.Radius, d.Height)
	if e != nil {
		return fail()
	}
	if e = ctx.Err(); e != nil {
		return QuarterCylinderSolid{}, e
	}
	return QuarterCylinderSolid{Descriptor: d, LengthUnit: doc.unit, Solid: solid, SourceSolidID: 69, SourceShellID: 67, SourceVertexListID: 25, SourceEdgeListID: 45, SourceFaceIDs: [5]int{57, 59, 61, 63, 65}, NativeFaceSourceIDs: [5]int{57, 61, 59, 63, 65}, SourceSurfaceIDs: [5]int{15, 17, 19, 21, 23}, Parameters: values, Metadata: metadata}, nil
}

// WriteQuarterCylinderSolid stages exact analytic IGES topology and typed
// re-import before writing. The file describes one material solid in model units.
func WriteQuarterCylinderSolid(ctx context.Context, w io.Writer, d QuarterCylinderDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return quarterCylinderFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := quarterCylinderAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadQuarterCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
