package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ClippedPolynomialFuseIGESOptions controls the exact native reproof and
// any decimal rounding of derived curve poles. PoleTolerance zero requires
// exact IGES decimals. A positive value bounds each pole coordinate error
// in its model or UV chart; exact weights preserve that bound on the curve.
type ClippedPolynomialFuseIGESOptions struct {
	Proof         edgecheck.Options
	PoleTolerance float64
}

// WriteClippedPolynomialFuse transfers a proved stepped Fuse as IGES type
// 186 material with type 128 supports and paired type 126 edge laws.
// The model coordinates and pcurve domains retain their original units.
func WriteClippedPolynomialFuse(ctx context.Context, w io.Writer, fuse *trimmed.ClippedPolynomialFuse, unit float64, l Limits, o ClippedPolynomialFuseIGESOptions) error {
	bound := new(big.Rat).SetFloat64(o.PoleTolerance)
	if ctx == nil || w == nil || fuse == nil || bound == nil || bound.Sign() < 0 {
		return geom.ErrInvalid
	}
	l, err := l.defaults()
	if err != nil {
		return err
	}
	s := fuse.Boundary()
	if len(s.Topology().Shells) != 1 {
		return geom.ErrConvergence
	}
	b, err := s.ExtractExactBezierBoundary(ctx, o.Proof)
	if err != nil {
		return err
	}
	a := topologyAuthor{}
	controls := 0
	consume := func(count int) error {
		if count > l.MaxControlPoints-controls {
			return geom.ErrConvergence
		}
		controls += count
		return ctx.Err()
	}
	supports := make([]int, len(b.Faces))
	for i, f := range b.Faces {
		s, err := f.BSplineSupport()
		if err != nil {
			return fmt.Errorf("iges: clipped Fuse support %d: %w", i, err)
		}
		d := s.Data()
		if err := consume(len(d.Poles)); err != nil {
			return err
		}
		us, ue, vs, ve := s.Domain()
		polynomial := true
		for _, w := range d.Weights {
			polynomial = polynomial && w == 1
		}
		en := Entity{Surface: &Surface{Geometry: s, UStart: us, UEnd: ue, VStart: vs, VEnd: ve, Polynomial: polynomial}}
		supports[i] = a.entity(en)
		record, err := exactClippedFuseIGESSupport(d, [4]float64{us, ue, vs, ve}, polynomial)
		if err != nil {
			return err
		}
		a.records[len(a.records)-1].data = record
	}
	verts := []string{"502", strconv.Itoa(len(b.Vertices))}
	for _, p := range b.Vertices {
		for _, v := range []float64{p.X, p.Y, p.Z} {
			x, err := clippedFuseIGESDecimal(new(big.Rat).SetFloat64(v))
			if err != nil {
				return err
			}
			verts = append(verts, x)
		}
	}
	vertexList := a.add(502, 1, joinTopologyValues(verts))
	edgeList := []int{504, len(b.Edges)}
	for i, e := range b.Edges {
		if err := consume(len(e.Model.Control)); err != nil {
			return err
		}
		v, err := exactClippedFuseIGESBezier(e.Model, false, bound)
		if err != nil {
			return fmt.Errorf("iges: clipped Fuse model edge %d: %w", i, err)
		}
		model := a.add(126, 0, v)
		edgeList = append(edgeList, model, vertexList, e.Start+1, vertexList, e.End+1)
	}
	edgeDir := a.add(504, 1, topologyNumbers(edgeList...))
	faces := []int{514, len(b.Faces)}
	incidence := make([]int, len(b.Edges))
	for i, f := range b.Faces {
		face := []int{510, supports[i], len(f.Loops), 1}
		for _, loop := range f.Loops {
			uses := []int{508, len(loop)}
			for _, u := range loop {
				if err := consume(len(u.UV.Control)); err != nil {
					return err
				}
				v, err := exactClippedFuseIGESBezier(u.UV, true, bound)
				if err != nil {
					return fmt.Errorf("iges: clipped Fuse edge %d face %d pcurve: %w", u.Edge, i, err)
				}
				pc := a.add(126, 0, v)
				uses = append(uses, 0, edgeDir, u.Edge+1, clippedFuseIGESOrientation(u.Reversed), 1, 0, pc)
				incidence[u.Edge]++
			}
			face = append(face, a.add(508, 1, topologyNumbers(uses...)))
		}
		faces = append(faces, a.add(510, 1, topologyNumbers(face...)), clippedFuseIGESOrientation(f.Reversed))
	}
	for _, n := range incidence {
		if n != 2 {
			return geom.ErrConvergence
		}
	}
	shell := a.add(514, 1, topologyNumbers(faces...))
	a.add(186, 0, topologyNumbers(186, shell, 1, 0))
	if len(a.records) > l.MaxEntities {
		return geom.ErrConvergence
	}
	raw, err := a.file(unit)
	if err != nil {
		return err
	}
	if _, err := readTopologySource(bytes.NewReader(raw), l, true); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}

// VerifyClippedPolynomialFuseIGES authenticates the canonical IGES topology
// and all original coedge laws against a source-proved native Fuse. It
// allows harmless start-section changes. Independent IGES import without
// the original source proof is outside this operation.
func VerifyClippedPolynomialFuseIGES(ctx context.Context, input io.Reader, fuse *trimmed.ClippedPolynomialFuse, unit float64, l Limits, o ClippedPolynomialFuseIGESOptions) error {
	if ctx == nil || input == nil || fuse == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actual, err := readTopologySource(input, l, true)
	if err != nil {
		return err
	}
	var canonical bytes.Buffer
	if err := WriteClippedPolynomialFuse(ctx, &canonical, fuse, unit, l, o); err != nil {
		return err
	}
	want, err := readTopologySource(&canonical, l, true)
	if err != nil {
		return err
	}
	if actual.unit != want.unit || !reflect.DeepEqual(actual.entities, want.entities) {
		return fmt.Errorf("iges: clipped Fuse topology or units differ from source proof: %w", geom.ErrConvergence)
	}
	return ctx.Err()
}

func clippedFuseIGESOrientation(reversed bool) int {
	if reversed {
		return 0
	}
	return 1
}

// IGES accepts finite decimal parameters. Use the exact represented surface
// values instead of the default shortest decimal float encoding.
func exactClippedFuseIGESSupport(d geom.BSplineSurfaceData, domain [4]float64, polynomial bool) (string, error) {
	p := []string{"128", strconv.Itoa(d.CountU - 1), strconv.Itoa(d.CountV - 1), strconv.Itoa(d.DegreeU), strconv.Itoa(d.DegreeV), "0", "0", flag(polynomial), "0", "0"}
	add := func(v ...float64) error {
		for _, x := range v {
			s, err := clippedFuseIGESDecimal(new(big.Rat).SetFloat64(x))
			if err != nil {
				return err
			}
			p = append(p, s)
		}
		return nil
	}
	if err := add(d.KnotsU...); err != nil {
		return "", err
	}
	if err := add(d.KnotsV...); err != nil {
		return "", err
	}
	for v := 0; v < d.CountV; v++ {
		for u := 0; u < d.CountU; u++ {
			if err := add(d.Weights[u*d.CountV+v]); err != nil {
				return "", err
			}
		}
	}
	for v := 0; v < d.CountV; v++ {
		for u := 0; u < d.CountU; u++ {
			x := d.Poles[u*d.CountV+v]
			if err := add(x.X, x.Y, x.Z); err != nil {
				return "", err
			}
		}
	}
	if err := add(domain[:]...); err != nil {
		return "", err
	}
	return joinTopologyValues(append(p, "0", "0")), nil
}

func exactClippedFuseIGESBezier(span geom.ExactBezierSpan, planar bool, tolerance *big.Rat) (string, error) {
	deg := len(span.Control) - 1
	one := big.NewRat(1, 1)
	weights := make([]*big.Rat, len(span.Control))
	anchor := span.Control[0][3]
	polynomial := true
	denominator := big.NewInt(1)
	for i, x := range span.Control {
		w := new(big.Rat).Quo(x[3], anchor)
		polynomial = polynomial && w.Cmp(one) == 0
		weights[i] = w
		d := w.Denom()
		gcd := new(big.Int).GCD(nil, nil, denominator, d)
		denominator.Mul(denominator.Quo(denominator, gcd), d)
		if denominator.BitLen() > 256 {
			return "", geom.ErrUnrepresentable
		}
	}
	p := []string{"126", strconv.Itoa(deg), strconv.Itoa(deg), flag(planar), "0", flag(polynomial), "0"}
	for range span.Control {
		p = append(p, "0.")
	}
	for range span.Control {
		p = append(p, "1.")
	}
	for _, w := range weights {
		s, err := clippedFuseIGESDecimal(w.Mul(w, new(big.Rat).SetInt(denominator)))
		if err != nil {
			return "", err
		}
		p = append(p, s)
	}
	for i, h := range span.Control {
		for j := 0; j < 3; j++ {
			q := new(big.Rat).Quo(h[j], h[3])
			x, err := clippedFuseIGESDecimal(q)
			if err != nil && i > 0 && i < deg && tolerance.Sign() > 0 {
				v, _ := q.Float64()
				nearest := new(big.Rat).SetFloat64(v)
				if nearest != nil && new(big.Rat).Abs(new(big.Rat).Sub(q, nearest)).Cmp(tolerance) <= 0 {
					x, err = clippedFuseIGESDecimal(nearest)
				}
			}
			if err != nil {
				return "", err
			}
			p = append(p, x)
		}
	}
	return joinTopologyValues(append(p, "0.", "1.", "0.", "0.", flag(planar)+".", "0", "0")), nil
}

func clippedFuseIGESDecimal(q *big.Rat) (string, error) {
	if q == nil {
		return "", geom.ErrInvalid
	}
	d := new(big.Int).Set(q.Denom())
	twos := int(d.TrailingZeroBits())
	d.Rsh(d, uint(twos))
	one, five := big.NewInt(1), big.NewInt(5)
	var rem big.Int
	fives := 0
	for d.Cmp(one) != 0 {
		d.QuoRem(d, five, &rem)
		fives++
		if rem.Sign() != 0 || fives > 256 {
			return "", geom.ErrUnrepresentable
		}
	}
	if twos > 256 {
		return "", geom.ErrUnrepresentable
	}
	x := q.FloatString(max(twos, fives))
	if !strings.Contains(x, ".") {
		x += "."
	}
	return x, nil
}
