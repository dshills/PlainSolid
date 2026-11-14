package step

import (
	"bytes"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CircularNAUORepairOptions explicitly authorizes this canonical reconstruction
// up to MaxDistance in source model units. It must be positive, finite, and no
// greater than one millionth of either source radius or height.
type CircularNAUORepairOptions struct{ MaxDistance float64 }

// CircularNAUORepairBounds measures the declared repair against original source
// sites, using exact decimal arithmetic and rational bounds for pi. These are
// squared Euclidean length bounds, unlike the inspection's squared-radius
// equation residuals. The bound concerns the mathematical analytic supports,
// not rounding of arbitrary floating-point evaluator calls.
type CircularNAUORepairBounds struct {
	Tolerance                              *big.Rat
	VertexDisplacementSquared              [2]*big.Rat
	SeamModelDisplacementSquared           *big.Rat
	SeamOwnerDisplacementSquaredUpper      [2]*big.Rat
	SeamModelOwnerDisplacementSquaredUpper [2]*big.Rat
}

// RepairedCircularNAUO is newly reconstructed local geometry, not source-identical
// imported material. Source retains its original failed conditions and IDs.
// The native cylinder adds polar-cap edges/vertices; only the primary original
// rim/seam and face roles are mapped below. There is no assembly pose conversion.
type RepairedCircularNAUO struct {
	Body                       CylinderBody
	Solid                      parametric.Solid
	LengthUnit                 float64
	Source                     CircularNAUOInspection
	Bounds                     CircularNAUORepairBounds
	NativeFaceSourceIDs        [3]int64 // side, lower cap, upper cap
	NativePrimaryEdgeSourceIDs [3]int64 // lower rim, seam, upper rim
	NativeRimVertexSourceIDs   [2]int64 // lower and upper zero-angle vertices
}

func circularRepairVector(v [3]*big.Rat) (geom.Vec3, error) {
	var a [3]float64
	for i, q := range v {
		var exact bool
		a[i], exact = q.Float64()
		if !exact || math.IsNaN(a[i]) || math.IsInf(a[i], 0) {
			return geom.Vec3{}, mappedPlanarFailure()
		}
	}
	return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, nil
}

// RepairCircularNAUOProduct explicitly chooses a complete canonical cylinder
// from the narrowly inspected original support/incidence family. It never changes
// support center, axes, radius or height, and fails unless their native values
// and all constructed corner/center points are losslessly representable.
// Source vertices and model seam are mapped to the zero-angle native seam;
// the two authored cylindrical seam branches are rephased to 2*pi and zero.
// Every displacement and model/owner disagreement bound must fit MaxDistance.
// This proves those repair correspondences, not original finite face trim
// selection, source equivalence, or a whole-source Hausdorff bound. Existing
// exact importers and inspection behavior are unchanged.
func RepairCircularNAUOProduct(r io.Reader, l Limits, productID int64, o CircularNAUORepairOptions) (RepairedCircularNAUO, error) {
	fail := func() (RepairedCircularNAUO, error) { return RepairedCircularNAUO{}, mappedPlanarFailure() }
	if r == nil || o.MaxDistance <= 0 || math.IsNaN(o.MaxDistance) || math.IsInf(o.MaxDistance, 0) {
		return fail()
	}
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, lim.MaxBytes+1))
	if e != nil {
		return RepairedCircularNAUO{}, e
	}
	if int64(len(raw)) > lim.MaxBytes {
		return fail()
	}
	source, e := InspectCircularNAUOProduct(bytes.NewReader(raw), l, productID)
	if e != nil {
		return RepairedCircularNAUO{}, e
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	cyl, e := simple(f, source.CylinderID, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	center, axes, e := sphereLiftFrame(f, cyl.Args[1].Ref)
	if e != nil {
		return fail()
	}
	radius, e := liftSourceRat(cyl.Args[2])
	if e != nil {
		return fail()
	}
	height := source.Rims[1].Height
	tolerance := new(big.Rat).SetFloat64(o.MaxDistance)
	cap := new(big.Rat).Quo(radius, big.NewRat(1000000, 1))
	if tolerance.Cmp(cap) > 0 {
		return fail()
	}
	cap.Quo(height, big.NewRat(1000000, 1))
	if tolerance.Cmp(cap) > 0 {
		return fail()
	}
	squared := new(big.Rat).Mul(tolerance, tolerance)
	bounds := CircularNAUORepairBounds{Tolerance: tolerance}
	ideal := [2][3]*big.Rat{}
	for i, rim := range source.Rims {
		for j := 0; j < 3; j++ {
			ideal[i][j] = new(big.Rat).Add(center[j], new(big.Rat).Add(new(big.Rat).Mul(radius, axes[0][j]), new(big.Rat).Mul(rim.Height, axes[2][j])))
		}
		p, e := circularNAUOVertex(f, rim.VertexID)
		if e != nil {
			return fail()
		}
		delta := circularNAUOSub(p, ideal[i])
		bounds.VertexDisplacementSquared[i] = mappedRationalDot(delta, delta)
		if bounds.VertexDisplacementSquared[i].Cmp(squared) > 0 {
			return fail()
		}
		if _, e = circularRepairVector(ideal[i]); e != nil {
			return fail()
		}
	}
	// Inspection proves the model LINE's origin is the lower source vertex,
	// direction equals the support axis and endpoint separation equals height.
	// Hence this constant vector difference applies to the entire finite seam.
	if bounds.VertexDisplacementSquared[0].Cmp(bounds.VertexDisplacementSquared[1]) != 0 {
		return fail()
	}
	bounds.SeamModelDisplacementSquared = new(big.Rat).Set(bounds.VertexDisplacementSquared[0])
	phase := new(big.Rat).Abs(source.TurnDefectLower)
	if q := new(big.Rat).Abs(source.TurnDefectUpper); q.Cmp(phase) > 0 {
		phase = q
	}
	// |(cos u,sin u)-(cos v,sin v)| <= |u-v| for all real u,v.
	phaseDistance := new(big.Rat).Mul(radius, phase)
	bounds.SeamOwnerDisplacementSquaredUpper = [2]*big.Rat{new(big.Rat).Mul(phaseDistance, phaseDistance), new(big.Rat)}
	for i, owner := range bounds.SeamOwnerDisplacementSquaredUpper {
		if owner.Cmp(squared) > 0 {
			return fail()
		}
		// (sqrt(a)+sqrt(b))^2 <= 2(a+b), without rounded square roots.
		together := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(bounds.SeamModelDisplacementSquared, owner))
		bounds.SeamModelOwnerDisplacementSquaredUpper[i] = together
		if together.Cmp(squared) > 0 {
			return fail()
		}
	}
	origin, e := circularRepairVector(center)
	if e != nil {
		return fail()
	}
	x, e := circularRepairVector(axes[0])
	if e != nil {
		return fail()
	}
	z, e := circularRepairVector(axes[2])
	if e != nil {
		return fail()
	}
	top := [3]*big.Rat{}
	for j := range top {
		top[j] = new(big.Rat).Add(center[j], new(big.Rat).Mul(height, axes[2][j]))
	}
	if _, e = circularRepairVector(top); e != nil {
		return fail()
	}
	rr, exact := radius.Float64()
	if !exact || rr <= 0 || math.IsInf(rr, 0) {
		return fail()
	}
	hh, exact := height.Float64()
	if !exact || hh <= 0 || math.IsInf(hh, 0) {
		return fail()
	}
	frame, e := geom.NewFrame(origin, x, z)
	if e != nil {
		return fail()
	}
	solid, e := parametric.Cylinder(frame, rr, hh)
	if e != nil {
		return fail()
	}
	return RepairedCircularNAUO{Body: CylinderBody{Name: source.Product.Name, Frame: frame, Radius: rr, Height: hh}, Solid: solid, LengthUnit: source.LengthUnit, Source: source, Bounds: bounds, NativeFaceSourceIDs: [3]int64{source.SideFaceID, source.Rims[0].CapFaceID, source.Rims[1].CapFaceID}, NativePrimaryEdgeSourceIDs: [3]int64{source.Rims[0].EdgeID, source.SeamEdgeID, source.Rims[1].EdgeID}, NativeRimVertexSourceIDs: [2]int64{source.Rims[0].VertexID, source.Rims[1].VertexID}}, nil
}
