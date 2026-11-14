package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strings"
)

// EllipseTabulated is a type122 swept closed directrix. End is the original
// model-space terminal point of the generatrix starting at the ring's first
// point. It is not a vector. No caps, finite face trims or solid are implied.
type EllipseTabulated struct {
	Directrix       EllipseConicRing
	End             geom.Vec3
	Metadata        CylinderDatumMetadata
	SourceSurfaceID int
}

// ExactPatches returns four exact rational tensor patches S(t,v)=C(t)+vD,
// t,v in[0,1]. The directrix parameter is each quarter's original half-angle
// reparameterization, and v is normalized displacement, not physical distance.
// This parameterization repeats at the outer U seam; it does not add topology.
func (m EllipseTabulated) ExactPatches(ctx context.Context, o geom.SurfacePatchOptions) ([4]*geom.SurfacePatch, error) {
	var out [4]*geom.SurfacePatch
	if !m.End.IsFinite() || cylinderMetadata(m.Metadata) != nil {
		return out, geom.ErrInvalid
	}
	// Prove the complete ring, including caller-supplied member agreement.
	var check bytes.Buffer
	if e := WriteEllipseConicRing(&check, m.Directrix); e != nil {
		return out, e
	}
	c, e := m.Directrix.Members[0].ExactCurve(ctx, o)
	if e != nil {
		return out, e
	}
	net, _, e := c.BezierControlNet(ctx, o)
	if e != nil {
		return out, e
	}
	end := [3]float64{m.End.X, m.End.Y, m.End.Z}
	var delta [3]*big.Rat
	nonzero := false
	for j := range delta {
		start := new(big.Rat).Quo(net.Control[0][j], net.Control[0][3])
		delta[j] = new(big.Rat).Sub(tetraRat(end[j]), start)
		nonzero = nonzero || delta[j].Sign() != 0
	}
	if !nonzero {
		return out, geom.ErrDomain
	}
	for i, q := range m.Directrix.Members {
		c, e := q.ExactCurve(ctx, o)
		if e != nil {
			return [4]*geom.SurfacePatch{}, e
		}
		net, _, e := c.BezierControlNet(ctx, o)
		if e != nil {
			return [4]*geom.SurfacePatch{}, e
		}
		h := make([][4]*big.Rat, 0, 6)
		for _, p := range net.Control {
			a, b := [4]*big.Rat{}, [4]*big.Rat{}
			for j := 0; j < 4; j++ {
				a[j] = new(big.Rat).Set(p[j])
				b[j] = new(big.Rat).Set(p[j])
				if j < 3 {
					b[j].Add(b[j], new(big.Rat).Mul(delta[j], p[3]))
				}
			}
			h = append(h, a, b)
		}
		out[i], e = geom.NewExactHomogeneousBezierSurface(ctx, 2, 1, h, o)
		if e != nil {
			return [4]*geom.SurfacePatch{}, e
		}
	}
	return out, nil
}

func ReadEllipseTabulated(r io.Reader, l Limits) (EllipseTabulated, error) {
	bad := func() (EllipseTabulated, error) {
		return EllipseTabulated{}, fmt.Errorf("iges: unsupported ellipse tabulated surface")
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 256)
	l.MaxEntities = min(l.MaxEntities, 7)
	l.MaxValues = min(l.MaxValues, 1024)
	d, e := readPlacedTopologyRecords(r, l, false, false, false, false, false, false, true, false, false, true, true, true)
	if e != nil {
		return EllipseTabulated{}, e
	}
	var root *topologyRecord
	var rest []topologyRecord
	for i := range d.entities {
		x := &d.entities[i]
		if x.kind == 122 {
			if root != nil {
				return bad()
			}
			root = x
		} else {
			rest = append(rest, *x)
		}
	}
	if root == nil || root.transform != 0 || len(root.values) != 5 {
		return bad()
	}
	if compositeSourceNumbers(110, root.values) != nil {
		return bad()
	}
	k, e := intValue(root.values[0])
	id, err := intValue(root.values[1])
	if e != nil || err != nil || k != 122 {
		return bad()
	}
	d.entities = rest
	ring, e := decodeEllipseConicRing(d, true)
	if e != nil || ring.SourceCompositeID != id {
		return bad()
	}
	m := EllipseTabulated{Directrix: ring, Metadata: CylinderDatumMetadata{root.label, root.subscript}, SourceSurfaceID: root.id}
	v := [3]float64{}
	for i := range v {
		v[i], e = realValue(root.values[i+2])
		if e != nil {
			return bad()
		}
	}
	m.End = geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	if _, e = m.ExactPatches(context.Background(), geom.SurfacePatchOptions{}); e != nil {
		return bad()
	}
	return m, nil
}

func WriteEllipseTabulated(w io.Writer, m EllipseTabulated) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if _, e := m.ExactPatches(context.Background(), geom.SurfacePatchOptions{}); e != nil {
		return e
	}
	var b bytes.Buffer
	if e := WriteEllipseConicRing(&b, m.Directrix); e != nil {
		return e
	}
	d, e := readPlacedTopologyRecords(bytes.NewReader(b.Bytes()), Limits{}, false, false, false, false, false, false, true, false, false, true, true)
	if e != nil {
		return e
	}
	a := topologyAuthor{}
	rid := 0
	for _, x := range d.entities {
		vals := []string{}
		for _, p := range x.values {
			vals = append(vals, p.text)
		}
		form := 0
		if x.kind == 104 {
			form = 1
		}
		id := a.add(x.kind, form, strings.Join(vals, ",")+";")
		if id != x.id {
			return geom.ErrInvalid
		}
		a.records[len(a.records)-1].label = x.label
		a.records[len(a.records)-1].subscript = x.subscript
		if x.kind == 102 {
			rid = id
		}
	}
	root := a.add(122, 0, fmt.Sprintf("122,%d,%s,%s,%s;", rid, number(m.End.X), number(m.End.Y), number(m.End.Z)))
	a.records[len(a.records)-1].label = m.Metadata.Label
	a.records[len(a.records)-1].subscript = m.Metadata.Subscript
	raw, e := a.file(m.Directrix.Members[0].LengthUnit)
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
		status := 10000
		if id == root {
			status = 0
		}
		for _, x := range d.entities {
			if x.id == id {
				copy(line[48:56], fmt.Sprintf("%8d", x.transform))
				if x.kind == 124 {
					status = 0
				}
			}
		}
		copy(line[64:72], fmt.Sprintf("%08d", status))
	}
	if _, e = ReadEllipseTabulated(bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
