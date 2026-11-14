package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strconv"
)

// CylinderTorusCavityBody describes cylinder minus an entirely enclosed torus.
// This analytic export accepts represented dyadic canonical dimensions of magnitude
// at most 2^20, requiring exact represented seam sums and axial differences.
// Periodic pcurve endpoints use represented 2pi as in the analytic primitives.
type CylinderTorusCavityBody struct {
	Name                                                   string
	MajorRadius, MinorRadius, CylinderRadius, Lower, Upper float64
}

// WriteCylinderTorusCavity emits one BREP_WITH_VOIDS: the outward cylinder
// CLOSED_SHELL and a reversed ORIENTED_CLOSED_SHELL of the complete torus.
// Geometry/preflight failures are atomic. No transverse dialect is modified.
func WriteCylinderTorusCavity(w io.Writer, b CylinderTorusCavityBody, unit float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	for _, x := range []float64{b.MajorRadius, b.MinorRadius, b.CylinderRadius, b.Lower, b.Upper} {
		if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1<<20 {
			return geom.ErrDomain
		}
	}

	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	for _, v := range []*big.Rat{new(big.Rat).Add(q(b.MajorRadius), q(b.MinorRadius)), new(big.Rat).Sub(q(b.Upper), q(b.Lower))} {
		if _, e := geom.RepresentedExactScalar(v); e != nil {
			return e
		}
	}
	if _, e := parametric.NewCylinderTorusCavitySet(context.Background(), b.MajorRadius, b.MinorRadius, b.CylinderRadius, b.Lower, b.Upper, 1e-9); e != nil {
		return e
	}
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	base, _ := geom.NewFrame(geom.Vec3{Z: b.Lower}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	var cylinder, torus bytes.Buffer
	if e := WriteCylinders(&cylinder, CylinderModel{Bodies: []CylinderBody{{b.Name, base, b.CylinderRadius, b.Upper - b.Lower}}, LengthUnit: unit}); e != nil {
		return e
	}
	if e := WriteTori(&torus, TorusModel{Bodies: []TorusBody{{b.Name, frame, b.MajorRadius, b.MinorRadius}}, LengthUnit: unit}); e != nil {
		return e
	}
	var files []*File
	for _, raw := range [][]byte{cylinder.Bytes(), torus.Bytes()} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			return e
		}

		// Primitive writer reals are round-trippable binary64 spellings. Expand
		// them to the exact dyadic values before graph composition, including
		// pcurve numbers; this avoids silently perturbing original supports.
		var expand func(Value) (Value, error)
		expand = func(v Value) (Value, error) {
			if v.Kind == Number {
				x, e := strconv.ParseFloat(v.Text, 64)
				if e != nil || math.IsInf(x, 0) || math.IsNaN(x) {
					return Value{}, geom.ErrInvalid
				}
				v.Text = torusCylinderSTEPReal(x)
			}
			for i, x := range v.Items {
				y, e := expand(x)
				if e != nil {
					return Value{}, e
				}
				v.Items[i] = y
			}
			return v, nil
		}
		for id, en := range f.Entities {
			for i := range en.Records {
				for j, v := range en.Records[i].Args {
					x, e := expand(v)
					if e != nil {
						return e
					}
					en.Records[i].Args[j] = x
				}
			}
			f.Entities[id] = en
		}
		files = append(files, f)
	}
	var pair bytes.Buffer
	e := writeStandalone(&pair, unit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		var roots []int64
		for _, f := range files {
			mapped := map[int64]int64{}
			var copyEntity func(int64) int64
			var walk func(Value)
			walk = func(v Value) {
				if v.Kind == Reference {
					copyEntity(v.Ref)
				}
				for _, x := range v.Items {
					walk(x)
				}
			}
			copyEntity = func(id int64) int64 {
				if n, ok := mapped[id]; ok {
					return n
				}
				en := f.Entities[id]
				for _, r := range en.Records {
					for _, v := range r.Args {
						walk(v)
					}
				}
				n := emit(assemblyEntity(en, mapped))
				mapped[id] = n
				return n
			}
			for _, id := range f.Order {
				if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
					roots = append(roots, copyEntity(id))
				}
			}
		}
		return roots
	})
	if e != nil {
		return e
	}
	f, e := Parse(&pair, Limits{})
	if e != nil {
		return e
	}
	out, e := joinWrappedVoid(f)
	if e != nil {
		return e
	}
	n, e := w.Write(out)
	if e == nil && n != len(out) {
		return io.ErrShortWrite
	}
	return e
}
