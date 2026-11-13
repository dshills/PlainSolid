package iges

import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// ReadCompositeCurves reads standalone ordered, direct type102 polycurves.
// Each of at most64 roots owns 1–64 distinct type100/110/126 leaves. Nesting,
// sharing, unowned geometry, surface topology and tolerant endpoint repair are
// unsupported. Original geometric decimal numbers must be exactly representable
// as float64; spline weight normalization and endpoint evaluation must be lossless.
func ReadCompositeCurves(r io.Reader, l Limits) (Model, error) {
	l, e := l.defaults()
	if e != nil {
		return Model{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxControlPoints = min(l.MaxControlPoints, 4096)
	m, e := readGeometryMode(r, l, false, true)
	if e != nil {
		return Model{}, e
	}
	if e = validateCompositeWires(m); e != nil {
		return Model{}, e
	}
	return m, nil
}

// WriteCompositeCurves validates the complete directed wire graph before writing.
// It retains original entity order, names, units and leaf transforms. Output is
// staged and checked against the same source-decimal contract as the reader.
func WriteCompositeCurves(w io.Writer, m Model) error {
	if w == nil {
		return errors.New("iges: nil writer")
	}
	if e := validateCompositeWires(m); e != nil {
		return e
	}
	var b bytes.Buffer
	if e := writeGeometryMode(&b, m, false, true); e != nil {
		return e
	}
	if _, e := ReadCompositeCurves(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		return io.ErrShortWrite
	}
	return e
}
func compositeSourceNumbers(kind int, values []parameter) error {
	switch kind {
	case 100, 102, 110, 124, 126:
	default:
		return errors.New("iges: unsupported standalone composite entity")
	}
	for _, v := range values {
		s := strings.ReplaceAll(strings.ReplaceAll(v.text, "D", "E"), "d", "E")
		if v.stringValue || len(s) == 0 || len(s) > 64 {
			return errors.New("iges: unsupported exact source scalar")
		}
		if i := strings.IndexAny(s, "eE"); i >= 0 {
			n, e := strconv.Atoi(s[i+1:])
			if e != nil || n < -256 || n > 256 {
				return errors.New("iges: exact scalar exponent limit")
			}
		}
		x, e := realValue(v)
		if e != nil {
			return e
		}
		q, ok := new(big.Rat).SetString(s)
		if !ok || q.Num().BitLen() > 2048 || q.Denom().BitLen() > 2048 || q.Cmp(new(big.Rat).SetFloat64(x)) != 0 {
			return errors.New("iges: source scalar would round")
		}
	}
	return nil
}
func compositeRat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
func compositeEndpoints(e Entity) ([2][3]*big.Rat, error) {
	var out [2][3]*big.Rat
	if err := e.validate(); err != nil {
		return out, err
	}
	if e.sourceWeightsRounded {
		return out, errors.New("iges: source weights rounded")
	}
	var p [2]geom.Vec3
	switch {
	case e.Line != nil:
		p = [2]geom.Vec3{e.Line.Start, e.Line.End}
	case e.Arc != nil:
		a := e.Arc
		p = [2]geom.Vec3{{X: a.Start.X, Y: a.Start.Y, Z: a.Center.Z}, {X: a.End.X, Y: a.End.Y, Z: a.Center.Z}}
		var radii [2]*big.Rat
		for i, q := range p {
			dx := new(big.Rat).Sub(compositeRat(q.X), compositeRat(a.Center.X))
			dy := new(big.Rat).Sub(compositeRat(q.Y), compositeRat(a.Center.Y))
			radii[i] = new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
		}
		if radii[0].Sign() <= 0 || radii[0].Cmp(radii[1]) != 0 {
			return out, errors.New("iges: unequal exact arc radii")
		}
	case e.Spline != nil:
		if len(e.Spline.Curve.Poles()) > 4096 {
			return out, errors.New("iges: spline pole limit")
		}
		c, err := geom.NewExactSplineCurve(e.Spline.Curve, geom.SurfacePatchOptions{MaxPoles: 4096, MaxOperations: 200000, MaxCoefficientBits: 4096})
		if err != nil {
			return out, err
		}
		for i, t := range []float64{e.Spline.Start, e.Spline.End} {
			p[i], err = c.ExactPoint(t)
			if err != nil {
				return out, err
			}
		}
	default:
		return out, errors.New("iges: unsupported wire leaf")
	}
	for i, q := range p {
		a := [3]*big.Rat{compositeRat(q.X), compositeRat(q.Y), compositeRat(q.Z)}
		for j := 0; j < 3; j++ {
			out[i][j] = new(big.Rat).Set(a[j])
			if e.Transform != nil {
				out[i][j] = compositeRat(e.Transform[j][3])
				for k := 0; k < 3; k++ {
					out[i][j].Add(out[i][j], new(big.Rat).Mul(compositeRat(e.Transform[j][k]), a[k]))
				}
			}
		}
	}
	return out, nil
}
func validateCompositeWires(m Model) error {
	if len(m.Entities) > 4096 || len(m.CurvesOnSurfaces) != 0 || len(m.TrimmedSurfaces) != 0 || len(m.Revolutions) != 0 {
		return errors.New("iges: composite graph limit or unsupported surfaces")
	}
	owned := map[int]bool{}
	roots, poles := 0, 0
	for i, e := range m.Entities {
		if e.Composite == nil {
			continue
		}
		roots++
		if roots > 64 {
			return errors.New("iges: composite root limit")
		}
		if err := m.validateCompositeMode(i, true); err != nil {
			return err
		}
		var last [3]*big.Rat
		for j, k := range e.Composite.Curves {
			if owned[k] {
				return errors.New("iges: shared composite leaf")
			}
			owned[k] = true
			leaf := m.Entities[k]
			if leaf.Spline != nil {
				poles += len(leaf.Spline.Curve.Poles())
				if poles > 4096 {
					return errors.New("iges: cumulative pole limit")
				}
			}
			ends, err := compositeEndpoints(leaf)
			if err != nil {
				return err
			}
			if j > 0 {
				for a := 0; a < 3; a++ {
					if last[a].Cmp(ends[0][a]) != 0 {
						return errors.New("iges: exact directed endpoint gap")
					}
				}
			}
			last = ends[1]
		}
	}
	if roots == 0 {
		return errors.New("iges: missing composite root")
	}
	for i, e := range m.Entities {
		if e.Composite == nil && !owned[i] {
			return errors.New("iges: unowned wire entity")
		}
	}
	return nil
}
