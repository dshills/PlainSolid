package iges

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strings"
)

// EllipseConic is one full IGES104 form1 model curve. Coefficients retain the
// original A,B,C,D,E,F in Ax²+Bxy+Cy²+Dx+Ey+F=0 at local Z. This bounded domain
// has B=D=E=0,0<A<C,F<0 and source start=end=(majorRadius,0).
// Placement is nil or a uniquely owned proper signed-coordinate type124.
type EllipseConic struct {
	Coefficients                [6]float64
	Z                           float64
	Placement                   *geom.Affine
	Metadata, PlacementMetadata CylinderDatumMetadata
}
type EllipseConicModel struct {
	Curve                            EllipseConic
	LengthUnit                       float64
	SourceCurveID, SourceTransformID int
}

func ellipseConicRadii(c EllipseConic) (float64, float64, error) {
	for _, v := range c.Coefficients {
		if !finite(v) {
			return 0, 0, geom.ErrInvalid
		}
	}
	a, b, z, d, e, f := c.Coefficients[0], c.Coefficients[1], c.Coefficients[2], c.Coefficients[3], c.Coefficients[4], c.Coefficients[5]
	if !finite(c.Z) || b != 0 || d != 0 || e != 0 || a <= 0 || z <= a || f >= 0 {
		return 0, 0, geom.ErrInvalid
	}
	root := func(q *big.Rat) (float64, error) {
		n, den := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
		if new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 || new(big.Int).Mul(den, den).Cmp(q.Denom()) != 0 {
			return 0, geom.ErrUnrepresentable
		}
		v, exact := new(big.Rat).SetFrac(n, den).Float64()
		if !exact || !finite(v) || v <= 0 {
			return 0, geom.ErrUnrepresentable
		}
		return v, nil
	}
	rx, err := root(new(big.Rat).Quo(new(big.Rat).Neg(tetraRat(f)), tetraRat(a)))
	if err != nil {
		return 0, 0, err
	}
	ry, err := root(new(big.Rat).Quo(new(big.Rat).Neg(tetraRat(f)), tetraRat(z)))
	return rx, ry, err
}

// Geometry preserves the natural positive angular parameter: t=0 is the local
// +X major endpoint, increasing t turns toward local +Y; a full turn is 2π.
// Numeric Eval follows geom.Ellipse conventions; no finite face trim is implied.
func (c EllipseConic) Geometry() (geom.Ellipse, error) {
	a, b, e := ellipseConicRadii(c)
	if e != nil {
		return geom.Ellipse{}, e
	}
	if e = cylinderMetadata(c.Metadata); e != nil {
		return geom.Ellipse{}, e
	}
	if e = cylinderMetadata(c.PlacementMetadata); e != nil {
		return geom.Ellipse{}, e
	}
	m := geom.Identity()
	if c.Placement != nil {
		m = *c.Placement
		if !roofRigid(m) {
			return geom.Ellipse{}, geom.ErrInvalid
		}
	} else if c.PlacementMetadata != (CylinderDatumMetadata{}) {
		return geom.Ellipse{}, geom.ErrInvalid
	}
	center, e := roofMapPoint(m, geom.Vec3{Z: c.Z}, false)
	if e != nil {
		return geom.Ellipse{}, e
	}
	if _, e = roofMapPoint(m, geom.Vec3{X: a, Z: c.Z}, false); e != nil {
		return geom.Ellipse{}, e
	}
	frame, e := geom.NewFrame(center, m.ApplyVector(geom.Vec3{X: 1}), m.ApplyVector(geom.Vec3{Z: 1}))
	if e != nil {
		return geom.Ellipse{}, e
	}
	return geom.NewEllipse(frame, a, b)
}

// ReadEllipseConic accepts one complete full ellipse and at most one owned124.
// Source scalars must be lossless binary64 decimals. Unsupported partial arcs,
// rotated local equations, affine scaling/reflection and ignored records fail.
func ReadEllipseConic(r io.Reader, l Limits) (EllipseConicModel, error) {
	return readEllipseConic(r, l, nil)
}
func readEllipseConic(r io.Reader, l Limits, quarter *int) (EllipseConicModel, error) {
	bad := func() (EllipseConicModel, error) {
		return EllipseConicModel{}, fmt.Errorf("iges: unsupported exact full ellipse conic")
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 16384)
	l.MaxRecords = min(l.MaxRecords, 128)
	l.MaxEntities = min(l.MaxEntities, 2)
	l.MaxValues = min(l.MaxValues, 256)
	doc, e := readPlacedTopologyRecords(r, l, false, false, false, false, false, false, true, false, false, true)
	if e != nil {
		return EllipseConicModel{}, e
	}
	for _, line := range bytes.Split(doc.bytes, []byte("\n")) {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		seq, e := field(string(line[73:]))
		if e != nil {
			return bad()
		}
		fields := []int{1, 2}
		if seq%2 == 1 {
			fields = []int{3, 4, 8}
		}
		for _, i := range fields {
			v, e := field(string(line[8*i : 8*i+8]))
			if e != nil || v != 0 {
				return bad()
			}
		}
	}
	return decodeEllipseConicRecords(doc, quarter)
}

func decodeEllipseConicRecords(doc topologySource, quarter *int) (EllipseConicModel, error) {
	bad := func() (EllipseConicModel, error) {
		return EllipseConicModel{}, fmt.Errorf("iges: unsupported exact ellipse conic")
	}
	var e error
	var curve, tr *topologyRecord
	for i := range doc.entities {
		x := &doc.entities[i]
		if x.kind == 104 {
			if curve != nil {
				return bad()
			}
			curve = x
		} else if x.kind == 124 {
			if tr != nil {
				return bad()
			}
			tr = x
		} else {
			return bad()
		}
	}
	if curve == nil || len(curve.values) != 12 {
		return bad()
	}
	if e = compositeSourceNumbers(110, curve.values); e != nil {
		return EllipseConicModel{}, e
	}
	k, e := intValue(curve.values[0])
	if e != nil || k != 104 {
		return bad()
	}
	c := EllipseConic{Metadata: CylinderDatumMetadata{curve.label, curve.subscript}}
	for i := range c.Coefficients {
		c.Coefficients[i], e = realValue(curve.values[i+1])
		if e != nil {
			return bad()
		}
	}
	c.Z, e = realValue(curve.values[7])
	if e != nil {
		return bad()
	}
	a, b, e := ellipseConicRadii(c)
	if e != nil {
		return EllipseConicModel{}, e
	}
	ends := [4]float64{}
	for i := range ends {
		ends[i], e = realValue(curve.values[8+i])
		if e != nil {
			return bad()
		}
	}
	if quarter == nil {
		if ends != [4]float64{a, 0, a, 0} {
			return bad()
		}
	} else {
		points := [4][2]float64{{a, 0}, {0, b}, {-a, 0}, {0, -b}}
		q := -1
		for i, p := range points {
			if ends[0] == p[0] && ends[1] == p[1] && ends[2] == points[(i+1)%4][0] && ends[3] == points[(i+1)%4][1] {
				q = i
			}
		}
		if q < 0 {
			return bad()
		}
		*quarter = q
	}

	tid := 0
	if curve.transform == 0 {
		if tr != nil {
			return bad()
		}
	} else {
		if tr == nil || curve.transform != tr.id || tr.transform != 0 || len(tr.values) != 13 {
			return bad()
		}
		if e = compositeSourceNumbers(124, tr.values); e != nil {
			return EllipseConicModel{}, e
		}
		kind, e := intValue(tr.values[0])
		if e != nil || kind != 124 {
			return bad()
		}
		m, e := decodeMatrix(tr.values)
		if e != nil || !roofRigid(m) {
			return bad()
		}
		c.Placement = &m
		c.PlacementMetadata = CylinderDatumMetadata{tr.label, tr.subscript}
		tid = tr.id
	}
	if _, e = c.Geometry(); e != nil {
		return EllipseConicModel{}, e
	}
	return EllipseConicModel{Curve: c, LengthUnit: doc.unit, SourceCurveID: curve.id, SourceTransformID: tid}, nil
}

// WriteEllipseConic retains raw coefficient scaling and declared placement.
// It re-reads the complete result before any output; failure is atomic.
func WriteEllipseConic(w io.Writer, m EllipseConicModel) error {
	return writeEllipseConic(w, m, -1)
}
func writeEllipseConic(w io.Writer, m EllipseConicModel, quarter int) error {
	if w == nil || !finite(m.LengthUnit) || m.LengthUnit <= 0 {
		return geom.ErrInvalid
	}
	if _, e := m.Curve.Geometry(); e != nil {
		return e
	}
	a, b, e := ellipseConicRadii(m.Curve)
	if e != nil {
		return e
	}
	author := topologyAuthor{}
	nums := []string{"104"}
	for _, v := range m.Curve.Coefficients {
		nums = append(nums, number(v))
	}
	ends := [4]float64{a, 0, a, 0}
	if quarter >= 0 {
		if quarter > 3 {
			return geom.ErrInvalid
		}
		points := [4][2]float64{{a, 0}, {0, b}, {-a, 0}, {0, -b}}
		p, q := points[quarter], points[(quarter+1)%4]
		ends = [4]float64{p[0], p[1], q[0], q[1]}
	}
	for _, v := range []float64{m.Curve.Z, ends[0], ends[1], ends[2], ends[3]} {
		nums = append(nums, number(v))
	}
	author.add(104, 1, strings.Join(nums, ",")+";")
	author.records[0].label = m.Curve.Metadata.Label
	author.records[0].subscript = m.Curve.Metadata.Subscript
	tid := 0
	if m.Curve.Placement != nil {
		nums = []string{"124"}
		for _, row := range *m.Curve.Placement {
			for _, v := range row {
				nums = append(nums, number(v))
			}
		}
		tid = author.add(124, 0, strings.Join(nums, ",")+";")
		author.records[1].label = m.Curve.PlacementMetadata.Label
		author.records[1].subscript = m.Curve.PlacementMetadata.Subscript
	}
	raw, e := author.file(m.LengthUnit)
	if e != nil {
		return e
	}
	lines := bytes.Split(raw, []byte("\n"))
	for _, line := range lines {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		seq, e := field(string(line[73:]))
		if e != nil {
			return e
		}
		if seq%2 == 1 {
			copy(line[64:72], []byte("00000000"))
		}
		if seq == 1 {
			copy(line[48:56], []byte(fmt.Sprintf("%8d", tid)))
		}
	}
	raw = bytes.Join(lines, []byte("\n"))
	var selected *int
	if quarter >= 0 {
		selected = new(int)
	}
	if _, e = readEllipseConic(bytes.NewReader(raw), Limits{}, selected); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
