package step

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RepairedEllipseExtrusionFace is a reconstructed full-period side face, not a
// solid or an exact import. Source retains the authored rounded seam phase.
// Native U is radians, V is physical distance along the negative profile axis;
// its rectangle is [0,2π] × [-Height,0]. Edge roles are seam, upper rim, lower
// rim. The two seam uses have distinct source pcurves and opposite senses.
type RepairedEllipseExtrusionFace struct {
	Name                                         string
	Face                                         trimmed.Face
	Profile                                      geom.Ellipse
	Height, MajorRadius, MinorRadius, LengthUnit float64
	Source                                       *File
	SourceFaceID, SourceSurfaceID                int64
	SourceEdgeIDs                                [3]int64
	SourceSeamPCurveIDs                          [2]int64
	Bounds                                       EllipticalWireRepairBounds
}

// RepairRoundedEllipseExtrusionFace authenticates the complete 89-entity OCCT
// ruled ellipse graph with cardinal frame, negative unit extrusion vector and
// two natural full ellipse rims. Only the second seam phase is repaired. For
// normalized chart s∈[0,1], the authored S(P*s,v) and repaired S(2π*s,v) differ
// by at most a*|P-2π| for every v. The same bound covers the displaced seam's
// mismatch with its authoritative model LINE. No authored finite LINE trim is
// inferred; endpoints and monotonic V fix the selected segment in the new face.
func RepairRoundedEllipseExtrusionFace(r io.Reader, l Limits, o EllipticalWireRepairOptions) (RepairedEllipseExtrusionFace, error) {
	fail := func() (RepairedEllipseExtrusionFace, error) {
		return RepairedEllipseExtrusionFace{}, fmt.Errorf("step: unsupported or over-tolerance elliptical extrusion face repair")
	}
	if r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 89)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return RepairedEllipseExtrusionFace{}, e
	}
	if len(f.Entities) != 89 {
		return fail()
	}
	roles := map[string]int64{}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		k := en.Records[0].Type
		switch k {
		case "SURFACE_OF_LINEAR_EXTRUSION", "SEAM_CURVE", "SHAPE_DEFINITION_REPRESENTATION", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_RELATED_PRODUCT_CATEGORY":
			if roles[k] != 0 {
				return fail()
			}
			roles[k] = id
		}
	}
	sr, e := simple(f, roles["SURFACE_OF_LINEAR_EXTRUSION"], "SURFACE_OF_LINEAR_EXTRUSION", 3)
	if e != nil {
		return fail()
	}
	pid, e := ref(sr.Args[1])
	if e != nil {
		return fail()
	}
	pr, e := simple(f, pid, "ELLIPSE", 4)
	if e != nil {
		return fail()
	}
	fid, e := ref(pr.Args[1])
	if e != nil {
		return fail()
	}
	frame, e := mappedExactFrame(f, fid)
	if e != nil {
		return fail()
	}
	a, e := cylinderSupportNumber(pr.Args[2])
	if e != nil {
		return fail()
	}
	b, e := cylinderSupportNumber(pr.Args[3])
	if e != nil || b <= 0 || a <= b {
		return fail()
	}
	seam, e := simple(f, roles["SEAM_CURVE"], "SEAM_CURVE", 4)
	if e != nil || seam.Args[2].Kind != Aggregate || len(seam.Args[2].Items) != 2 {
		return fail()
	}
	pcid, e := ref(seam.Args[2].Items[1])
	if e != nil {
		return fail()
	}
	pc, e := simple(f, pcid, "PCURVE", 3)
	if e != nil {
		return fail()
	}
	rid, e := ref(pc.Args[2])
	if e != nil {
		return fail()
	}
	rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil || len(rep.Args[1].Items) != 1 {
		return fail()
	}
	lid, e := ref(rep.Args[1].Items[0])
	if e != nil {
		return fail()
	}
	line, e := simple(f, lid, "LINE", 3)
	if e != nil {
		return fail()
	}
	oid, e := ref(line.Args[1])
	if e != nil {
		return fail()
	}
	point, e := simple(f, oid, "CARTESIAN_POINT", 2)
	if e != nil || len(point.Args[1].Items) != 2 {
		return fail()
	}
	period, e := liftSourceRat(point.Args[1].Items[0])
	if e != nil {
		return fail()
	}
	// The sole nonclosed source edge determines signed physical height exactly.
	var ends [2]geom.Vec3
	found := false
	for _, en := range f.Entities {
		if len(en.Records) != 1 || en.Records[0].Type != "EDGE_CURVE" {
			continue
		}
		er := en.Records[0]
		if len(er.Args) != 5 {
			return fail()
		}
		i, e := ref(er.Args[1])
		if e != nil {
			return fail()
		}
		j, e := ref(er.Args[2])
		if e != nil {
			return fail()
		}
		if i == j {
			continue
		}
		if found {
			return fail()
		}
		found = true
		for k, id := range []int64{i, j} {
			vr, e := simple(f, id, "VERTEX_POINT", 2)
			if e != nil {
				return fail()
			}
			v, e := ref(vr.Args[1])
			if e != nil {
				return fail()
			}
			ends[k], e = mappedExactCoordinate(f, v, "CARTESIAN_POINT", 3)
			if e != nil {
				return fail()
			}
		}
	}
	if !found {
		return fail()
	}
	z := frame.ZAxis()
	vals := func(v geom.Vec3) []float64 { return []float64{v.X, v.Y, v.Z} }
	height := new(big.Rat)
	for i, c := range vals(z) {
		d := new(big.Rat).Sub(new(big.Rat).SetFloat64(vals(ends[1])[i]), new(big.Rat).SetFloat64(vals(ends[0])[i]))
		height.Add(height, d.Mul(d, new(big.Rat).SetFloat64(c)))
	}
	h, exact := height.Float64()
	if !exact || !finiteNumber(h) || h <= 0 {
		return fail()
	}
	tolerance := new(big.Rat).SetFloat64(o.MaxDistance)
	if tolerance.Cmp(new(big.Rat).Quo(new(big.Rat).SetFloat64(b), big.NewRat(1000000, 1))) > 0 {
		return fail()
	}
	lo, hi := pcurvePiBounds()
	lo.Mul(lo, big.NewRat(2, 1))
	hi.Mul(hi, big.NewRat(2, 1))
	dl := new(big.Rat).Sub(period, hi)
	dh := new(big.Rat).Sub(period, lo)
	delta := new(big.Rat).Abs(dl)
	if x := new(big.Rat).Abs(dh); x.Cmp(delta) > 0 {
		delta = x
	}
	distance := new(big.Rat).Mul(new(big.Rat).SetFloat64(a), delta)
	if distance.Cmp(tolerance) > 0 {
		return fail()
	}
	var buf bytes.Buffer
	analyticSupportHeader(&buf)
	buf.WriteString(ellipseExtrusionTemplate)
	buf.WriteString("ENDSEC;END-ISO-10303-21;")
	want, e := Parse(&buf, Limits{})
	if e != nil {
		return fail()
	}
	number := func(x float64) Value {
		s, e := mappedTerminatingDecimal(new(big.Rat).SetFloat64(x))
		if e != nil {
			return Value{}
		}
		return Value{Kind: Number, Text: s}
	}
	setVec := func(id int64, v geom.Vec3) {
		en := want.Entities[id]
		en.Records[0].Args[1].Items = []Value{number(v.X), number(v.Y), number(v.Z)}
		want.Entities[id] = en
	}
	// All required world points must themselves be losslessly representable.
	sum := func(base, dir geom.Vec3, scale float64) (geom.Vec3, bool) {
		v := [3]float64{}
		for i, c := range vals(base) {
			q := new(big.Rat).Add(new(big.Rat).SetFloat64(c), new(big.Rat).Mul(new(big.Rat).SetFloat64(vals(dir)[i]), new(big.Rat).SetFloat64(scale)))
			var ok bool
			v[i], ok = q.Float64()
			if !ok || !finiteNumber(v[i]) {
				return geom.Vec3{}, false
			}
		}
		return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, true
	}
	top, ok := sum(frame.Origin(), z, h)
	if !ok {
		return fail()
	}
	bottomVertex, ok := sum(frame.Origin(), frame.XAxis(), a)
	if !ok {
		return fail()
	}
	topVertex, ok := sum(top, frame.XAxis(), a)
	if !ok {
		return fail()
	}
	for _, x := range []struct {
		id int64
		v  geom.Vec3
	}{{23, bottomVertex}, {25, topVertex}, {28, bottomVertex}, {30, z}, {35, frame.Origin()}, {36, z}, {37, frame.XAxis()}, {39, z.Scale(-1)}, {58, top}, {59, z}, {60, frame.XAxis()}, {74, frame.Origin()}, {75, z}, {76, frame.XAxis()}} {
		setVec(x.id, x.v)
	}
	for _, id := range []int64{33, 56, 72} {
		en := want.Entities[id]
		en.Records[0].Args[2] = pr.Args[2]
		en.Records[0].Args[3] = pr.Args[3]
		want.Entities[id] = en
	}
	en := want.Entities[49]
	en.Records[0].Args[1].Items[0] = point.Args[1].Items[0]
	want.Entities[49] = en
	en = want.Entities[64]
	en.Records[0].Args[1].Items[1] = number(-h)
	want.Entities[64] = en
	mapping, ok := ellipseFaceGraph(want, f, [][2]int64{{1, roles["APPLICATION_PROTOCOL_DEFINITION"]}, {3, roles["SHAPE_DEFINITION_REPRESENTATION"]}, {89, roles["PRODUCT_RELATED_PRODUCT_CATEGORY"]}})
	if !ok || mapping[32] != roles["SURFACE_OF_LINEAR_EXTRUSION"] || mapping[26] != roles["SEAM_CURVE"] {
		return fail()
	}
	unit, e := contextUnits(f, mapping[84])
	if e != nil || unit != .001 {
		return fail()
	}
	profile, e := geom.NewEllipse(frame, a, b)
	if e != nil {
		return fail()
	}
	identity, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail()
	}
	cylinder, e := geom.NewCylinderSurface(identity, 1)
	if e != nil {
		return fail()
	}
	var affine geom.Affine
	for i := range 3 {
		affine[i] = [4]float64{vals(frame.XAxis())[i] * a, vals(frame.YAxis())[i] * b, -vals(z)[i], vals(frame.Origin())[i]}
	}
	u := 2 * math.Pi
	face := trimmed.Face{Surface: parametric.MapSurface(cylinder, affine), Outer: []trimmed.Use{{Edge: 0, UVStart: geom.Vec2{}, UVEnd: geom.Vec2{Y: -h}}, {Edge: 1, UVStart: geom.Vec2{Y: -h}, UVEnd: geom.Vec2{X: u, Y: -h}}, {Edge: 0, Reversed: true, UVStart: geom.Vec2{X: u, Y: -h}, UVEnd: geom.Vec2{X: u}}, {Edge: 2, Reversed: true, UVStart: geom.Vec2{X: u}, UVEnd: geom.Vec2{}}}}
	product, _ := simple(f, mapping[7], "PRODUCT", 4)
	return RepairedEllipseExtrusionFace{Name: product.Args[1].Text, Face: face, Profile: profile, Height: h, MajorRadius: a, MinorRadius: b, LengthUnit: unit, Source: f, SourceFaceID: mapping[17], SourceSurfaceID: mapping[32], SourceEdgeIDs: [3]int64{mapping[21], mapping[54], mapping[70]}, SourceSeamPCurveIDs: [2]int64{mapping[31], mapping[46]}, Bounds: EllipticalWireRepairBounds{SourceStart: new(big.Rat), SourceEnd: new(big.Rat).Set(period), TwoPiLower: lo, TwoPiUpper: hi, SignedEndResidual: [2]*big.Rat{dl, dh}, DisplacementUpper: distance, Tolerance: tolerance}}, nil
}
