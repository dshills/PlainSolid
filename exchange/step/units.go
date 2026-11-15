package step

import (
	"errors"
	"fmt"
	"math"
	"math/big"
)

var unitPrefixes = map[int]string{0: "$", 18: ".EXA.", 15: ".PETA.", 12: ".TERA.", 9: ".GIGA.", 6: ".MEGA.", 3: ".KILO.", 2: ".HECTO.", 1: ".DECA.", -1: ".DECI.", -2: ".CENTI.", -3: ".MILLI.", -6: ".MICRO.", -9: ".NANO.", -12: ".PICO.", -15: ".FEMTO.", -18: ".ATTO."}

func validLengthUnit(u float64) bool { return u > 0 && !math.IsInf(u, 0) && !math.IsNaN(u) }
func emitLengthUnit(emit func(string) int64, u float64) int64 {
	return emitLengthUnitWithFormat(emit, u, real)
}

func emitExactPlanarLengthUnit(emit func(string) int64, u float64) int64 {
	return emitLengthUnitWithFormat(emit, u, func(u float64) string {
		switch u {
		case .0254:
			return "0.0254"
		case .3048:
			return "0.3048"
		default:
			return exactVertexContactPointText(u)
		}
	})
}

func emitLengthUnitWithFormat(emit func(string) int64, u float64, format func(float64) string) int64 {
	for p, n := range unitPrefixes {
		if u == math.Pow10(p) {
			return emit("(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(" + n + ",.METRE.))")
		}
	}
	metre := emit("(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT($,.METRE.))")
	measure := emit(fmt.Sprintf("MEASURE_WITH_UNIT(LENGTH_MEASURE(%s),#%d)", format(u), metre))
	dim := emit("DIMENSIONAL_EXPONENTS(1.,0.,0.,0.,0.,0.,0.)")
	name := "model unit"
	if u == .0254 {
		name = "inch"
	}
	if u == .3048 {
		name = "foot"
	}
	return emit(fmt.Sprintf("(CONVERSION_BASED_UNIT(%s,#%d) LENGTH_UNIT() NAMED_UNIT(#%d))", quote(name), measure, dim))
}

func lengthUnit(f *File, id int64) (float64, error) {
	return lengthUnitWithExactPlanar(f, id, false)
}

func exactPlanarPhysicalUnit(u float64) *big.Rat {
	for power := range unitPrefixes {
		if u == math.Pow10(power) {
			q, _ := new(big.Rat).SetString(fmt.Sprintf("1e%d", power))
			return q
		}
	}
	switch u {
	case .0254:
		return big.NewRat(127, 5000)
	case .3048:
		return big.NewRat(381, 1250)
	default:
		return new(big.Rat).SetFloat64(u)
	}
}

func lengthUnitWithExactPlanar(f *File, id int64, exact bool) (float64, error) {
	seen := map[int64]bool{}
	var visit func(int64, int) (*big.Rat, error)
	fail := func() (*big.Rat, error) { return nil, errors.New("step: invalid or cyclic length conversion") }
	visit = func(id int64, depth int) (*big.Rat, error) {
		if depth > 64 || seen[id] {
			return fail()
		}
		seen[id] = true
		ent := f.Entities[id]
		length, ok := component(ent, "LENGTH_UNIT")
		named, nok := component(ent, "NAMED_UNIT")
		if !ok || len(length.Args) != 0 || !nok || len(named.Args) != 1 || len(ent.Records) != 3 {
			return fail()
		}
		if si, ok := component(ent, "SI_UNIT"); ok {
			if named.Args[0].Kind != Derived || len(si.Args) != 2 || si.Args[1].Kind != Enumeration || si.Args[1].Text != "METRE" {
				return fail()
			}
			power := 0
			if si.Args[0].Kind != Omitted {
				if si.Args[0].Kind != Enumeration {
					return fail()
				}
				found := false
				for p, n := range unitPrefixes {
					if n == "."+si.Args[0].Text+"." {
						power = p
						found = true
					}
				}
				if !found {
					return fail()
				}
			}
			r, _ := new(big.Rat).SetString(fmt.Sprintf("1e%d", power))
			return r, nil
		}
		c, ok := component(ent, "CONVERSION_BASED_UNIT")
		if !ok || len(c.Args) != 2 {
			return fail()
		}
		if _, e := textValue(c.Args[0]); e != nil {
			return fail()
		}
		dim, e := ref(named.Args[0])
		if e != nil {
			return fail()
		}
		d, e := simple(f, dim, "DIMENSIONAL_EXPONENTS", 7)
		if e != nil {
			return fail()
		}
		for i, v := range d.Args {
			want := int64(0)
			if i == 0 {
				want = 1
			}
			if exact {
				x, e := exactVertexContactScalar(v)
				if e != nil || x.Cmp(big.NewRat(want, 1)) != 0 {
					return fail()
				}
			} else {
				x, e := numeric(v)
				if e != nil || x != float64(want) {
					return fail()
				}
			}
		}
		mid, e := ref(c.Args[1])
		if e != nil {
			return fail()
		}
		measureKind := "MEASURE_WITH_UNIT"
		if ent := f.Entities[mid]; len(ent.Records) == 1 && ent.Records[0].Type == "LENGTH_MEASURE_WITH_UNIT" {
			measureKind = "LENGTH_MEASURE_WITH_UNIT"
		}
		m, e := simple(f, mid, measureKind, 2)
		if e != nil {
			return fail()
		}
		number := m.Args[0]
		if measureKind == "MEASURE_WITH_UNIT" {
			v := m.Args[0]
			if v.Kind != Typed || v.Text != "LENGTH_MEASURE" || len(v.Items) != 1 {
				return fail()
			}
			number = v.Items[0]
		}

		var factor *big.Rat
		if exact {
			factor, e = exactVertexContactScalar(number)
			if e != nil || factor.Sign() <= 0 {
				return fail()
			}
		} else {
			if len(number.Text) > 1024 {
				return fail()
			}
			x, e := numeric(number)
			if e != nil || x <= 0 {
				return fail()
			}
			var ok bool
			factor, ok = new(big.Rat).SetString(number.Text)
			if !ok {
				return fail()
			}
		}
		base, e := ref(m.Args[1])
		if e != nil {
			return fail()
		}
		r, e := visit(base, depth+1)
		if e != nil {
			return nil, e
		}
		r.Mul(r, factor)
		if exact && (r.Num().BitLen() > 8192 || r.Denom().BitLen() > 8192) {
			return fail()
		}
		return r, nil
	}
	r, e := visit(id, 0)
	if e != nil {
		return 0, e
	}
	u, _ := r.Float64()
	if !validLengthUnit(u) {
		return 0, errors.New("step: length conversion outside float64 range")
	}
	if exact && r.Cmp(exactPlanarPhysicalUnit(u)) != 0 {
		return 0, errors.New("step: exact planar physical unit would change under represented transfer")
	}
	return u, nil
}
