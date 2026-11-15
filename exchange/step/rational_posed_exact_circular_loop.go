package step

import (
	"bytes"
	"context"
	"io"
	"math/big"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/geom"
)

const rationalPosedExactCircularLoopProduct = "rational-posed-exact-circular-loop-extrusion"
const localExactCircularLoopProduct = "exact-circular-loop-extrusion"

// RationalPosedExactCircularLoopExtrusionProduct retains the original
// canonical world graph and its original role IDs. Descriptor and Native
// are product-local. Pose maps either directly into the world graph.
type RationalPosedExactCircularLoopExtrusionProduct struct {
	ExactCircularLoopExtrusionProduct
	Pose rigidplacement.RationalRigid
}

// WriteRationalPosedExactCircularLoopExtrusion emits the existing canonical
// analytic loop graph in an exact rational world pose. The descriptor frame
// is the identity: Pose already accounts for the full placement. The
// product uses a distinct identifier and exact terminating STEP decimals.
func WriteRationalPosedExactCircularLoopExtrusion(ctx context.Context, w io.Writer, d ExactCircularLoopExtrusion, pose rigidplacement.RationalRigid) error {
	if ctx == nil || w == nil || frameTransform(d.Frame) != geom.Identity() {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if _, e := pose.Matrix(); e != nil {
		return e
	}
	var staged bytes.Buffer
	if e := WriteExactCircularLoopExtrusion(ctx, &staged, d); e != nil {
		return e
	}
	f, rep, _, bodies, e := parseCanonicalCircularExtrusions(&staged, Limits{})
	if e != nil {
		return e
	}
	if len(bodies) != 1 {
		return geom.ErrDomain
	}
	if d.LengthUnit == .0254 {
		if e = changeRationalLoopInchFactor(f, rep, real(.0254), "0.0254"); e != nil {
			return e
		}
	}
	body, e := nauoClosure(f, bodies, 16)
	if e != nil {
		return e
	}
	world, e := rationalConvexArcView(ctx, f, body, nil, pose, false)
	if e != nil {
		return e
	}
	if e = setExactCircularLoopProductIdentifier(world, localExactCircularLoopProduct, rationalPosedExactCircularLoopProduct); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(world)
	if e != nil {
		return e
	}
	l, e := occurrenceColorSourceLimits(Limits{})
	if e != nil {
		return e
	}
	if _, e = Parse(bytes.NewReader(raw), l); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadRationalPosedExactCircularLoopExtrusion recovers exact source placement
// from the low cap and the first wall station, transforms the complete model
// closure back to product coordinates, and authenticates the whole graph with
// the existing exact canonical reader. This reader applies no repair tolerance.
func ReadRationalPosedExactCircularLoopExtrusion(ctx context.Context, r io.Reader, l Limits) (RationalPosedExactCircularLoopExtrusionProduct, error) {
	var zero RationalPosedExactCircularLoopExtrusionProduct
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	f, rep, unit, bodies, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return zero, e
	}
	if len(bodies) != 1 {
		return zero, geom.ErrDomain
	}
	pose, e := readRationalExactCircularLoopPose(f, bodies[0])
	if e != nil {
		return zero, e
	}
	body, e := nauoClosure(f, bodies, 16)
	if e != nil {
		return zero, e
	}
	local, e := rationalConvexArcView(ctx, f, body, nil, pose, true)
	if e != nil {
		return zero, e
	}
	if e = setExactCircularLoopProductIdentifier(local, rationalPosedExactCircularLoopProduct, localExactCircularLoopProduct); e != nil {
		return zero, e
	}
	if unit == .0254 {
		// The world graph must declare the exact named unit. Translate only
		// that metadata in the private view used to authenticate the legacy
		// local graph, whose writer formats units through binary64.
		if e = changeRationalLoopInchFactor(local, rep, "0.0254", real(.0254)); e != nil {
			return zero, e
		}
	}
	normalized, e := mappedPlanarSourceBytes(local)
	if e != nil {
		return zero, e
	}
	out, e := ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(normalized), l)
	if e != nil {
		return zero, e
	}
	if frameTransform(out.Descriptor.Frame) != geom.Identity() {
		return zero, geom.ErrDomain
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	out.Source = f
	return RationalPosedExactCircularLoopExtrusionProduct{ExactCircularLoopExtrusionProduct: out, Pose: pose}, nil
}

func changeRationalLoopInchFactor(f *File, rep int64, from, to string) error {
	r, e := simple(f, rep, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil || r.Args[2].Kind != Reference {
		return geom.ErrDomain
	}
	units, ok := component(f.Entities[r.Args[2].Ref], "GLOBAL_UNIT_ASSIGNED_CONTEXT")
	if !ok || len(units.Args) != 1 {
		return geom.ErrDomain
	}
	ids, e := listRefs(units.Args[0], 3)
	if e != nil || len(ids) != 3 {
		return geom.ErrDomain
	}
	var measureID int64
	for _, id := range ids {
		c, ok := component(f.Entities[id], "CONVERSION_BASED_UNIT")
		if !ok {
			continue
		}
		if measureID != 0 || len(c.Args) != 2 || c.Args[0].Kind != String || c.Args[0].Text != "inch" || c.Args[1].Kind != Reference {
			return geom.ErrDomain
		}
		measureID = c.Args[1].Ref
	}
	m, e := simple(f, measureID, "MEASURE_WITH_UNIT", 2)
	if e != nil || m.Args[0].Kind != Typed || m.Args[0].Text != "LENGTH_MEASURE" || len(m.Args[0].Items) != 1 {
		return geom.ErrDomain
	}
	actual, e := liftSourceRat(m.Args[0].Items[0])
	want, ok := new(big.Rat).SetString(from)
	if e != nil || !ok || actual.Cmp(want) != 0 {
		return geom.ErrDomain
	}
	m.Args = append([]Value(nil), m.Args...)
	m.Args[0].Items = append([]Value(nil), m.Args[0].Items...)
	m.Args[0].Items[0].Text = to
	f.Entities[measureID] = Entity{ID: measureID, Records: []Record{m}}
	return nil
}

func setExactCircularLoopProductIdentifier(f *File, from, to string) error {
	var id int64
	var product Record
	for _, candidate := range f.Order {
		p, e := simple(f, candidate, "PRODUCT", 4)
		if e != nil {
			continue
		}
		if id != 0 || p.Args[0].Kind != String || p.Args[0].Text != from {
			return geom.ErrDomain
		}
		id, product = candidate, p
	}
	if id == 0 {
		return geom.ErrDomain
	}
	product.Args = append([]Value(nil), product.Args...)
	product.Args[0].Text = to
	f.Entities[id] = Entity{ID: id, Records: []Record{product}}
	return nil
}

func readRationalExactCircularLoopPose(f *File, body int64) (rigidplacement.RationalRigid, error) {
	var zero rigidplacement.RationalRigid
	solid, e := simple(f, body, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return zero, e
	}
	shell, e := simple(f, solid.Args[1].Ref, "CLOSED_SHELL", 2)
	if e != nil {
		return zero, e
	}
	faces, e := listRefs(shell.Args[1], 5)
	if e != nil || len(faces) > 66 {
		return zero, geom.ErrDomain
	}
	cap, e := simple(f, faces[0], "ADVANCED_FACE", 4)
	if e != nil {
		return zero, e
	}
	plane, e := simple(f, cap.Args[2].Ref, "PLANE", 2)
	if e != nil {
		return zero, e
	}
	low, x, y, z, e := convexUVFrame(f, plane.Args[1].Ref)
	if e != nil {
		return zero, e
	}
	wall, e := simple(f, faces[2], "ADVANCED_FACE", 4)
	if e != nil {
		return zero, e
	}
	support := f.Entities[wall.Args[2].Ref].Records
	if len(support) != 1 || (support[0].Type != "PLANE" || len(support[0].Args) != 2) &&
		(support[0].Type != "CYLINDRICAL_SURFACE" || len(support[0].Args) != 3) {
		return zero, geom.ErrDomain
	}
	wallOrigin, _, _, e := occtLoopFrame(f, support[0].Args[1].Ref)
	if e != nil {
		return zero, e
	}
	// The canonical wall supports are anchored at station zero. Their
	// projection onto the exact cap axis recovers the actual origin for
	// both signs of extrusion height.
	offset := occtLoopDot(occtLoopSub(wallOrigin, low), z)
	var matrix [3][4]*big.Rat
	for i := range matrix {
		o := new(big.Rat).Add(low[i], new(big.Rat).Mul(offset, z[i]))
		matrix[i] = [4]*big.Rat{x[i], y[i], z[i], o}
	}
	return rigidplacement.NewRationalRigid(matrix, 512)
}
