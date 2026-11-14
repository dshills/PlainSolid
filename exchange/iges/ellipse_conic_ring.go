package iges

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

// EllipseConicRing is an ordered closed model wire, not a face. Each member
// retains its original metadata; all four use the same exact support/placement.
type EllipseConicRing struct {
	Members           [4]QuarterEllipseConic
	Metadata          CylinderDatumMetadata
	SourceCompositeID int
}

// ReadEllipseConicRing accepts one direct102 owning four distinct positive
// quarter104 members, and optionally one shared proper signed-coordinate124.
func ReadEllipseConicRing(r io.Reader, l Limits) (EllipseConicRing, error) {
	bad := func() (EllipseConicRing, error) {
		return EllipseConicRing{}, fmt.Errorf("iges: unsupported exact ellipse ring")
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 256)
	l.MaxEntities = min(l.MaxEntities, 6)
	l.MaxValues = min(l.MaxValues, 1024)
	d, e := readPlacedTopologyRecords(r, l, false, false, false, false, false, false, true, false, false, true, true)
	if e != nil {
		return EllipseConicRing{}, e
	}
	var root, tr *topologyRecord
	leaves := map[int]topologyRecord{}
	for i := range d.entities {
		x := &d.entities[i]
		switch x.kind {
		case 102:
			if root != nil {
				return bad()
			}
			root = x
		case 124:
			if tr != nil {
				return bad()
			}
			tr = x
		case 104:
			leaves[x.id] = *x
		default:
			return bad()
		}
	}
	if root == nil || len(leaves) != 4 || len(root.values) != 6 || root.transform != 0 {
		return bad()
	}
	for _, line := range bytes.Split(d.bytes, []byte("\n")) {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		id, err := field(string(line[73:]))
		if err != nil {
			return bad()
		}
		indices := []int{1, 2}
		if id%2 == 1 {
			indices = []int{3, 4}
			status, err := field(string(line[64:72]))
			want := 0
			if _, ok := leaves[id]; ok {
				want = 10000
			}
			if err != nil || status != want {
				return bad()
			}
		}
		for _, j := range indices {
			v, err := field(string(line[j*8 : j*8+8]))
			if err != nil || v != 0 {
				return bad()
			}
		}
	}
	k, e := intValue(root.values[0])
	n, en := intValue(root.values[1])
	if e != nil || en != nil || k != 102 || n != 4 {
		return bad()
	}
	out := EllipseConicRing{Metadata: CylinderDatumMetadata{root.label, root.subscript}, SourceCompositeID: root.id}
	if cylinderMetadata(out.Metadata) != nil {
		return bad()
	}
	seen := map[int]bool{}
	for i := 0; i < 4; i++ {
		id, err := intValue(root.values[i+2])
		x, ok := leaves[id]
		if err != nil || !ok || seen[id] {
			return bad()
		}
		seen[id] = true
		part := topologySource{unit: d.unit, entities: []topologyRecord{x}}
		if tr != nil {
			part.entities = append(part.entities, *tr)
		}
		q := 0
		m, err := decodeEllipseConicRecords(part, &q)
		if err != nil {
			return bad()
		}
		out.Members[i] = QuarterEllipseConic{EllipseConicModel: m, StartQuarter: q}
		if i > 0 {
			a, b := out.Members[0], out.Members[i]
			if q != (a.StartQuarter+i)%4 || a.Curve.Coefficients != b.Curve.Coefficients || a.Curve.Z != b.Curve.Z || a.SourceTransformID != b.SourceTransformID {
				return bad()
			}
		}
	}
	return out, nil
}

// WriteEllipseConicRing stages and validates all directed members before output.
func WriteEllipseConicRing(w io.Writer, m EllipseConicRing) error {
	if w == nil || cylinderMetadata(m.Metadata) != nil {
		return geom.ErrInvalid
	}
	a := topologyAuthor{}
	ids := []string{}
	var placement *geom.Affine
	tid := 0
	for i, q := range m.Members {
		if q.StartQuarter < 0 || q.StartQuarter > 3 {
			return geom.ErrInvalid
		}
		if i > 0 {
			p := m.Members[0]
			if q.LengthUnit != p.LengthUnit || q.StartQuarter != (p.StartQuarter+i)%4 || q.Curve.Coefficients != p.Curve.Coefficients || q.Curve.Z != p.Curve.Z || q.Curve.PlacementMetadata != p.Curve.PlacementMetadata {
				return geom.ErrInvalid
			}
			if (q.Curve.Placement == nil) != (placement == nil) || placement != nil && *q.Curve.Placement != *placement {
				return geom.ErrInvalid
			}
		} else {
			placement = q.Curve.Placement
		}
		if _, e := q.Curve.Geometry(); e != nil {
			return e
		}
		x, y, e := ellipseConicRadii(q.Curve)
		if e != nil {
			return e
		}
		pts := [4][2]float64{{x, 0}, {0, y}, {-x, 0}, {0, -y}}
		p, t := pts[q.StartQuarter], pts[(q.StartQuarter+1)%4]
		v := []string{"104"}
		for _, n := range q.Curve.Coefficients {
			v = append(v, number(n))
		}
		for _, n := range []float64{q.Curve.Z, p[0], p[1], t[0], t[1]} {
			v = append(v, number(n))
		}
		id := a.add(104, 1, strings.Join(v, ",")+";")
		ids = append(ids, fmt.Sprint(id))
		a.records[i].label = q.Curve.Metadata.Label
		a.records[i].subscript = q.Curve.Metadata.Subscript
	}
	if placement != nil {
		v := []string{"124"}
		for _, row := range *placement {
			for _, n := range row {
				v = append(v, number(n))
			}
		}
		tid = a.add(124, 0, strings.Join(v, ",")+";")
		a.records[4].label = m.Members[0].Curve.PlacementMetadata.Label
		a.records[4].subscript = m.Members[0].Curve.PlacementMetadata.Subscript
	}
	root := a.add(102, 0, "102,4,"+strings.Join(ids, ",")+";")
	a.records[len(a.records)-1].label = m.Metadata.Label
	a.records[len(a.records)-1].subscript = m.Metadata.Subscript
	raw, e := a.file(m.Members[0].LengthUnit)
	if e != nil {
		return e
	}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		id, _ := field(string(line[73:]))
		if id%2 == 0 {
			continue
		}
		if id <= 7 {
			copy(line[48:56], fmt.Sprintf("%8d", tid))
		} else {
			copy(line[64:72], []byte("00000000"))
		}
		if id == root {
			copy(line[64:72], []byte("00000000"))
		}
	}
	if _, e = ReadEllipseConicRing(bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
