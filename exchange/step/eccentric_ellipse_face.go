package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
)

// EccentricEllipseClearance proves strict containment of the complete inner
// ellipse using its enclosing UV rectangle. ClearanceLower is a conservative
// Euclidean distance to the outer boundary in model units, not a sampled gap.
// Rational fields are owned copies. Center and Reference preserve exact UV data.
type EccentricEllipseClearance struct {
	Center, Reference                   geom.Vec2
	BoxNormSquaredUpper, ClearanceLower *big.Rat
}

// EccentricEllipticalFace adds one strictly eccentric, cardinally rotated hole
// to the independently verified planar ellipse face. It is not a solid.
type EccentricEllipticalFace struct {
	EllipticalAnnulusFace
	Clearance EccentricEllipseClearance
}

func eccentricEllipseFailure() error {
	return fmt.Errorf("step: unsupported or unproved eccentric elliptical face")
}
func eccentricUVStart(a, b geom.Vec2) (geom.Vec2, error) {
	out := [2]float64{}
	for i, v := range [][2]float64{{a.X, b.X}, {a.Y, b.Y}} {
		q := new(big.Rat).Add(new(big.Rat).SetFloat64(v[0]), new(big.Rat).SetFloat64(v[1]))
		f, exact := q.Float64()
		if !exact || !finiteNumber(f) {
			return geom.Vec2{}, eccentricEllipseFailure()
		}
		out[i] = f
	}
	return geom.Vec2{X: out[0], Y: out[1]}, nil
}
func eccentricEllipseClearance(outer, inner geom.Frame, a, b, c, d float64) (EccentricEllipseClearance, error) {
	fail := func() (EccentricEllipseClearance, error) {
		return EccentricEllipseClearance{}, eccentricEllipseFailure()
	}
	if inner.ZAxis() != outer.ZAxis() {
		return fail()
	}
	o := mappedRatVec(outer.Origin())
	p := mappedRatVec(inner.Origin())
	axes := []geom.Vec3{outer.XAxis(), outer.YAxis(), outer.ZAxis()}
	var q [3]*big.Rat
	for j, axis := range axes {
		v := []float64{axis.X, axis.Y, axis.Z}
		q[j] = new(big.Rat)
		for i := range 3 {
			q[j].Add(q[j], new(big.Rat).Mul(new(big.Rat).Sub(p[i], o[i]), new(big.Rat).SetFloat64(v[i])))
		}
	}
	if q[2].Sign() != 0 || (q[0].Sign() == 0 && q[1].Sign() == 0) {
		return fail()
	}
	x, ex := q[0].Float64()
	y, ey := q[1].Float64()
	if !ex || !ey || !finiteNumber(x) || !finiteNumber(y) {
		return fail()
	}
	rx := inner.XAxis().Dot(outer.XAxis())
	ry := inner.XAxis().Dot(outer.YAxis())
	if !(rx == 0 && (ry == 1 || ry == -1) || ry == 0 && (rx == 1 || rx == -1)) {
		return fail()
	}
	extents := [2]float64{c, d}
	if rx == 0 {
		extents = [2]float64{d, c}
	}
	norm := new(big.Rat)
	for i, r := range []float64{a, b} {
		v := new(big.Rat).Add(new(big.Rat).Abs(q[i]), new(big.Rat).SetFloat64(extents[i]))
		v.Quo(v, new(big.Rat).SetFloat64(r))
		norm.Add(norm, new(big.Rat).Mul(v, v))
	}
	if norm.Cmp(big.NewRat(1, 1)) >= 0 {
		return fail()
	}
	gap := new(big.Rat).Mul(new(big.Rat).SetFloat64(b), new(big.Rat).Sub(big.NewRat(1, 1), norm))
	gap.Quo(gap, big.NewRat(2, 1))
	return EccentricEllipseClearance{Center: geom.Vec2{X: x, Y: y}, Reference: geom.Vec2{X: rx, Y: ry}, BoxNormSquaredUpper: norm, ClearanceLower: gap}, nil
}

// ReadEccentricEllipticalFace accepts the genuine OCCT two-bound owner graph
// with exact eccentric coplanar centers and cardinal in-plane reference axes.
// A conservative exact rectangle-in-ellipse certificate is required; valid
// cases outside that certificate are rejected rather than sampled or repaired.
func ReadEccentricEllipticalFace(r io.Reader, l Limits) (EccentricEllipticalFace, error) {
	var proof EccentricEllipseClearance
	f, e := readEllipticalAnnulusFace(r, l, &proof)
	if e != nil {
		return EccentricEllipticalFace{}, e
	}
	return EccentricEllipticalFace{EllipticalAnnulusFace: f, Clearance: proof}, nil
}

// WriteEccentricEllipticalFaceSource revalidates and emits the original graph.
func WriteEccentricEllipticalFaceSource(w io.Writer, f *File, l Limits) error {
	if w == nil || !ellipseOwnedSourceBound(f, 66) {
		return eccentricEllipseFailure()
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadEccentricEllipticalFace(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
