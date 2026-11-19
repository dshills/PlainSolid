package step

import (
	"bytes"
	"io"
	"math/big"
	"plainsolid/geom"
	"runtime"
)

// RationalMappedPlacement authors a proper rigid occurrence from rational origin
// and raw X/Z ratios. Inputs are copied; no caller rational is retained. Ratios
// must be exactly perpendicular with rational lengths. STEP scalar decimals must
// terminate exactly. ItemID identifies an existing source mapped occurrence.
type RationalMappedPlacement struct {
	ItemID               int64
	Origin, XAxis, ZAxis [3]*big.Rat
}

// ExactMappedOccurrence owns an immutable exact rational rigid pose. Its source
// target and item IDs refer to the original graph, not the private proof view.
type ExactMappedOccurrence struct {
	Name                    string
	ItemID, TargetID, MapID int64
	pose                    mappedRationalBasis
}

func (o ExactMappedOccurrence) Matrix() (m [3][4]*big.Rat) {
	for i := range m {
		for j, a := range [][3]*big.Rat{o.pose.X, o.pose.Y, o.pose.Z, o.pose.Origin} {
			if a[i] != nil {
				m[i][j] = new(big.Rat).Set(a[i])
			}
			runtime.KeepAlive(&a) // golang/go#82026: keeps a in the stack map. Remove once the Go fix is in use.
		}
	}
	return
}

// ApplyExact returns independently owned world coordinates. The zero occurrence
// is invalid and returns an error, rather than fabricating an identity pose.
func (o ExactMappedOccurrence) ApplyExact(p geom.Vec3) (q [3]*big.Rat, e error) {
	if !p.IsFinite() {
		return q, mappedPlanarFailure()
	}
	m := o.Matrix()
	v := mappedRatVec(p)
	for i := range q {
		if m[i][3] == nil {
			return q, mappedPlanarFailure()
		}
		q[i] = new(big.Rat).Set(m[i][3])
		for j := range v {
			if m[i][j] == nil {
				return q, mappedPlanarFailure()
			}
			q[i].Add(q[i], new(big.Rat).Mul(m[i][j], v[j]))
		}
	}
	return
}

// ApplyRepresented returns a point only when no binary64 rounding is required.
func (o ExactMappedOccurrence) ApplyRepresented(p geom.Vec3) (geom.Vec3, error) {
	q, e := o.ApplyExact(p)
	if e != nil {
		return geom.Vec3{}, e
	}
	var v [3]float64
	for i := range v {
		var exact bool
		v[i], exact = q[i].Float64()
		if !exact {
			return geom.Vec3{}, mappedPlanarFailure()
		}
	}
	p = geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	if !p.IsFinite() {
		return geom.Vec3{}, mappedPlanarFailure()
	}
	return p, nil
}

// RationalMappedPlanarMaterials retains exact local native bodies and separate
// exact occurrences. It deliberately has no approximate geom.Affine cache.
type RationalMappedPlanarMaterials struct {
	Source      *File
	Name        string
	LengthUnit  float64
	Bodies      map[int64]Body
	MapOrder    []int64
	Occurrences []ExactMappedOccurrence
}

func mappedRationalPose(p RationalMappedPlacement) (b mappedRationalBasis, e error) {
	for j, a := range [][3]*big.Rat{p.Origin, p.XAxis, p.ZAxis} {
		for i, q := range a {
			if q == nil || q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
				return b, mappedPlanarFailure()
			}
			switch j {
			case 0:
				b.Origin[i] = new(big.Rat).Set(q)
			case 1:
				b.X[i] = new(big.Rat).Set(q)
			case 2:
				b.Z[i] = new(big.Rat).Set(q)
			}
		}
	}
	if mappedRationalDot(b.X, b.Z).Sign() != 0 {
		return b, mappedPlanarFailure()
	}
	b.X, e = mappedRationalUnit(b.X)
	if e != nil {
		return b, e
	}
	b.Z, e = mappedRationalUnit(b.Z)
	if e != nil {
		return b, e
	}
	b.Y = mappedRationalCross(b.Z, b.X)
	return
}
func mappedRationalOperator(f *File, id int64) (b mappedRationalBasis, e error) {
	r, e := simple(f, id, "CARTESIAN_TRANSFORMATION_OPERATOR_3D", 8)
	if e != nil {
		return b, e
	}
	for i := 0; i < 3; i++ {
		if r.Args[i].Kind != String || r.Args[i].Text != "" {
			return b, mappedPlanarFailure()
		}
	}
	scale, e := cylinderSupportNumber(r.Args[6])
	if e != nil || scale != 1 {
		return b, mappedPlanarFailure()
	}
	o, e := mappedRationalCoordinate(f, r.Args[5].Ref, "CARTESIAN_POINT", 3)
	if e != nil {
		return b, e
	}
	x, e := mappedRationalCoordinate(f, r.Args[3].Ref, "DIRECTION", 3)
	if e != nil {
		return b, e
	}
	z, e := mappedRationalCoordinate(f, r.Args[7].Ref, "DIRECTION", 3)
	if e != nil {
		return b, e
	}
	b, e = mappedRationalPose(RationalMappedPlacement{Origin: o, XAxis: x, ZAxis: z})
	if e != nil {
		return b, e
	}
	y, e := mappedRationalCoordinate(f, r.Args[4].Ref, "DIRECTION", 3)
	if e != nil {
		return b, e
	}
	y, e = mappedRationalUnit(y)
	if e != nil {
		return b, e
	}
	for i := range y {
		if y[i].Cmp(b.Y[i]) != 0 {
			return b, mappedPlanarFailure()
		}
	}
	return
}

// ReadRationalMappedPlanarMaterials validates the original rational target graph
// and uses a private identity-target view for the unchanged local material proof.
// No source geometry is repaired and no approximate world body is constructed.
func ReadRationalMappedPlanarMaterials(r io.Reader, l Limits) (RationalMappedPlanarMaterials, error) {
	fail := func() (RationalMappedPlanarMaterials, error) {
		return RationalMappedPlanarMaterials{}, mappedPlanarFailure()
	}
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return RationalMappedPlanarMaterials{}, e
	}
	var rootID, next int64
	count := 0
	for _, id := range f.Order {
		if id > next {
			next = id
		}
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_REPRESENTATION" {
			rootID = id
			count++
		}
	}
	if count != 1 || next > 1<<60 {
		return fail()
	}
	root, e := simple(f, rootID, "SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	items, e := listRefs(root.Args[1], 1)
	if e != nil || len(items) > 16 {
		return fail()
	}
	// Validate complete original reachability before dropping superseded operator
	// dependencies in the proof view. Detached geometry cannot disappear unnoticed.
	closure, e := mappedPlanarPartition(f, rootID, items, lim)
	if e != nil || len(closure.Entities) != len(f.Entities) {
		return fail()
	}
	out := RationalMappedPlanarMaterials{Source: f}
	for _, id := range items {
		item, e := simple(f, id, "MAPPED_ITEM", 3)
		if e != nil {
			return fail()
		}
		pose, e := mappedRationalOperator(f, item.Args[2].Ref)
		if e != nil {
			return fail()
		}
		name, e := textValue(item.Args[0])
		if e != nil {
			return fail()
		}
		out.Occurrences = append(out.Occurrences, ExactMappedOccurrence{Name: name, ItemID: id, TargetID: item.Args[2].Ref, MapID: item.Args[1].Ref, pose: pose})
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return fail()
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	emit := func(kind string, args ...Value) int64 {
		next++
		view.Order = append(view.Order, next)
		view.Entities[next] = Entity{ID: next, Records: []Record{{Type: kind, Args: args}}}
		return next
	}
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	empty := Value{Kind: String}
	coord := func(kind string, a, b, c string) int64 {
		return emit(kind, empty, Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: a}, {Kind: Number, Text: b}, {Kind: Number, Text: c}}})
	}
	for _, id := range items {
		p := coord("CARTESIAN_POINT", "0", "0", "0")
		x := coord("DIRECTION", "1", "0", "0")
		y := coord("DIRECTION", "0", "1", "0")
		z := coord("DIRECTION", "0", "0", "1")
		target := emit("CARTESIAN_TRANSFORMATION_OPERATOR_3D", empty, empty, empty, refv(x), refv(y), refv(p), Value{Kind: Number, Text: "1"}, refv(z))
		en := view.Entities[id]
		en.Records[0].Args[2] = refv(target)
		view.Entities[id] = en
	}
	view, e = mappedPlanarPartition(view, rootID, items, lim)
	if e != nil {
		return fail()
	}
	raw, e = mappedPlanarSourceBytes(view)
	if e != nil {
		return fail()
	}
	local, e := ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	out.Name, out.LengthUnit, out.MapOrder = local.Name, local.LengthUnit, local.MapOrder
	out.Bodies = map[int64]Body{}
	for mid, m := range local.Materials {
		out.Bodies[mid] = m.Body
	}
	return out, nil
}
