package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
)

// EllipticalWireRepairOptions explicitly permits replacing a source near-full
// parameter interval by the complete natural ellipse. MaxDistance is in model
// units, positive, finite, and at most one millionth of the minor radius.
type EllipticalWireRepairOptions struct{ MaxDistance float64 }

// EllipticalWireRepairBounds contains exact source decimals and certified bounds
// for mathematical curves, not floating-point evaluator roundoff. For normalized
// s in [0,1], Source(s)=E(s*SourceEnd), Natural(s)=E(s*2π). Their distance is at
// most DisplacementUpper. The same bound covers the source end's disagreement
// with its Cartesian selector; the source start selector agrees exactly.
// SignedEndResidual encloses SourceEnd−2π. No exact source equivalence is claimed.
type EllipticalWireRepairBounds struct {
	SourceStart, SourceEnd       *big.Rat
	TwoPiLower, TwoPiUpper       *big.Rat
	SignedEndResidual            [2]*big.Rat
	DisplacementUpper, Tolerance *big.Rat
}

// RepairedEllipticalWire is a new full ellipse, not an exact import of the
// authored finite interval. Original and Natural.Source are independently owned
// source graphs. Removed trim/selector IDs remain explicit provenance.
type RepairedEllipticalWire struct {
	Natural              EllipticalWire
	Original             *File
	TrimID               int64
	CartesianSelectorIDs [2]int64
	Bounds               EllipticalWireRepairBounds
}

// RepairRoundedEllipticalWire accepts only the strict original 29-record OCCT
// single-ellipse graph with forward PARAMETER trimming, start exactly zero and
// two distinct, exactly coincident Cartesian selectors at the +major point.
// It proves support/ownership through the unchanged natural-ellipse reader and
// bounds the complete same-normalized-parameter displacement with rational π.
// The discarded near-seam overlap/gap is an explicit reconstruction, never an
// inference that the original authored interval was exactly one full turn.
func RepairRoundedEllipticalWire(r io.Reader, l Limits, o EllipticalWireRepairOptions) (RepairedEllipticalWire, error) {
	fail := func() (RepairedEllipticalWire, error) {
		return RepairedEllipticalWire{}, fmt.Errorf("step: unsupported or over-tolerance rounded elliptical wire reconstruction")
	}
	if r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 29)
	l.MaxValues = min(l.MaxValues, 2048)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return RepairedEllipticalWire{}, e
	}
	if len(f.Entities) != 29 {
		return fail()
	}
	var tid, sid int64
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		switch en.Records[0].Type {
		case "TRIMMED_CURVE":
			if tid != 0 {
				return fail()
			}
			tid = id
		case "GEOMETRIC_CURVE_SET":
			if sid != 0 {
				return fail()
			}
			sid = id
		}
	}
	tr, e := simple(f, tid, "TRIMMED_CURVE", 6)
	if e != nil {
		return fail()
	}
	if tr.Args[0].Kind != String || len(tr.Args[0].Text) > 1024 {
		return fail()
	}
	basis, e := ref(tr.Args[1])
	if e != nil {
		return fail()
	}
	sense, e := boolean(tr.Args[4])
	if e != nil || !sense || tr.Args[5].Kind != Enumeration || tr.Args[5].Text != "PARAMETER" {
		return fail()
	}
	var points [2]int64
	var ends [2]*big.Rat
	for i := range 2 {
		v := tr.Args[2+i]
		if v.Kind != Aggregate || len(v.Items) != 2 {
			return fail()
		}
		for _, a := range v.Items {
			switch a.Kind {
			case Reference:
				if points[i] != 0 {
					return fail()
				}
				points[i] = a.Ref
			case Typed:
				if a.Text != "PARAMETER_VALUE" || len(a.Items) != 1 || ends[i] != nil {
					return fail()
				}
				ends[i], e = liftSourceRat(a.Items[0])
				if e != nil {
					return fail()
				}
			default:
				return fail()
			}
		}
		if points[i] == 0 || ends[i] == nil {
			return fail()
		}
	}
	if points[0] == points[1] || ends[0].Sign() != 0 || ends[1].Sign() <= 0 {
		return fail()
	}
	set, e := simple(f, sid, "GEOMETRIC_CURVE_SET", 2)
	if e != nil {
		return fail()
	}
	members, e := listRefs(set.Args[1], 1)
	if e != nil || len(members) != 1 || members[0] != tid {
		return fail()
	}
	// Drop only exclusive trim/selector records. Any surviving reference to one
	// of them, or any other owner/geometry variation, fails the exact re-read.
	g := &File{Header: f.Header, Entities: map[int64]Entity{}}
	for _, id := range f.Order {
		if id == tid || id == points[0] || id == points[1] {
			continue
		}
		g.Order = append(g.Order, id)
		g.Entities[id] = f.Entities[id]
	}
	en := g.Entities[sid]
	en.Records = append([]Record(nil), en.Records...)
	en.Records[0].Args = append([]Value(nil), en.Records[0].Args...)
	en.Records[0].Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: basis}}}
	g.Entities[sid] = en
	raw, e := mappedPlanarSourceBytes(g)
	if e != nil {
		return fail()
	}
	natural, e := ReadEllipticalWire(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	if natural.EllipseID != basis || natural.SetID != sid {
		return fail()
	}
	// Compare exact original Cartesian coordinates to center+major*reference;
	// neither represented vector addition nor a trig endpoint is used as proof.
	origin := []float64{natural.Location.X, natural.Location.Y, natural.Location.Z}
	axis := []float64{natural.Reference.X, natural.Reference.Y, natural.Reference.Z}
	for _, id := range points {
		p, e := simple(f, id, "CARTESIAN_POINT", 2)
		if e != nil || p.Args[0].Kind != String || len(p.Args[0].Text) > 1024 || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 3 {
			return fail()
		}
		for j, v := range p.Args[1].Items {
			q, e := liftSourceRat(v)
			if e != nil {
				return fail()
			}
			want := new(big.Rat).Add(new(big.Rat).SetFloat64(origin[j]), new(big.Rat).Mul(new(big.Rat).SetFloat64(natural.MajorRadius), new(big.Rat).SetFloat64(axis[j])))
			if q.Cmp(want) != 0 {
				return fail()
			}
		}
	}
	tolerance := new(big.Rat).SetFloat64(o.MaxDistance)
	ceiling := new(big.Rat).Quo(new(big.Rat).SetFloat64(natural.MinorRadius), big.NewRat(1000000, 1))
	if tolerance.Cmp(ceiling) > 0 {
		return fail()
	}
	lo, hi := pcurvePiBounds()
	lo.Mul(lo, big.NewRat(2, 1))
	hi.Mul(hi, big.NewRat(2, 1))
	deltaLo := new(big.Rat).Sub(ends[1], hi)
	deltaHi := new(big.Rat).Sub(ends[1], lo)
	absolute := new(big.Rat).Abs(deltaLo)
	other := new(big.Rat).Abs(deltaHi)
	if other.Cmp(absolute) > 0 {
		absolute = other
	}
	distance := new(big.Rat).Mul(new(big.Rat).SetFloat64(natural.MajorRadius), absolute)
	if distance.Cmp(tolerance) > 0 {
		return fail()
	}
	return RepairedEllipticalWire{Natural: natural, Original: f, TrimID: tid, CartesianSelectorIDs: points, Bounds: EllipticalWireRepairBounds{SourceStart: new(big.Rat).Set(ends[0]), SourceEnd: new(big.Rat).Set(ends[1]), TwoPiLower: lo, TwoPiUpper: hi, SignedEndResidual: [2]*big.Rat{deltaLo, deltaHi}, DisplacementUpper: distance, Tolerance: tolerance}}, nil
}
