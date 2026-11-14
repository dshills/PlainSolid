package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RoofBoxCommonHistory names an original owner for each retained output face.
// At coincident constraints the first source owns the output, deterministically.
// ProjectedContact records footprint contact independently of material volume.
type RoofBoxCommonHistory struct {
	Faces            []MixedRoofFaceOwner
	ProjectedContact bool
	ExactOperations  int
}

// IntersectOrderedRoofBoxes builds a native regularized Common when the two
// roofs have a provably fixed order over the entire common XY rectangle. Its
// initial construction lane requires a single rational Bézier span on the
// selected lower roof. Its geometry is preserved by exact restriction and
// ruled side faces. A changing order, roof/floor pinches, or
// unrepresentable model vertices are returned as unsupported. Disjoint or
// proved zero-volume results are empty. The exact arithmetic allowance bounds
// chart restriction/selection and construction. Continuous edge checks spend
// the remaining allowance independently for each original use.
func IntersectOrderedRoofBoxes(ctx context.Context, a, b Solid, proof MixedRoofQueryOptions, options geom.SurfacePatchEvaluationOptions) (Solid, RoofBoxCommonHistory, error) {
	var empty Solid
	var zero RoofBoxCommonHistory
	q, err := prepareDirectRoofBoxMeasure(ctx, a, b, parametric.Intersection, proof)
	if err != nil {
		return empty, zero, err
	}
	o, err := q.evaluationOptions(ctx, options)
	if err != nil {
		return empty, zero, err
	}
	if o.MaxOperations < 8 { // minimum allowance for the two exact footprints
		return empty, zero, geom.ErrConvergence
	}
	work := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o}}
	for _, stock := range []Solid{a, b} {
		if err := roofBoxCheckSourceEdges(ctx, stock, &work); err != nil {
			return empty, zero, err
		}
	}
	left, right := q.box[0], q.box[1]
	d := intersectRectangles(left.domain, right.domain)
	if !d.Valid() {
		return empty, RoofBoxCommonHistory{ProjectedContact: d.UMin <= d.UMax && d.VMin <= d.VMax}, nil
	}
	history := RoofBoxCommonHistory{ProjectedContact: true}
	xy := [2]geom.ExactParameterInterval{
		{Lower: ratInset(d.UMin), Upper: ratInset(d.UMax)},
		{Lower: ratInset(d.VMin), Upper: ratInset(d.VMax)},
	}
	floorIndex := 0
	if right.floor.Cmp(left.floor) > 0 {
		floorIndex = 1
	}
	floor := q.box[floorIndex].floor
	if len(left.roofSpans) != 0 || len(right.roofSpans) != 0 {
		return empty, zero, geom.ErrUnrepresentable
	}
	cache := make(roofOrderCache)
	order, err := cache.order(q.box, xy, &work)
	if err != nil {
		return empty, zero, err
	}
	roofIndex := 0
	if order.Upper.Sign() > 0 {
		sign := 1
		if order.Lower.Sign() < 0 {
			var err error
			sign, err = proveRoofBoxPowerOrder(ctx, cache[q.box].power, &work)
			if err != nil {
				return empty, zero, err
			}
		}
		if sign == 0 {
			return empty, zero, geom.ErrUnrepresentable
		}
		if sign > 0 {
			roofIndex = 1
		}
	}
	roof := q.box[roofIndex]
	raw, err := roofBoxRestrictedPatch(roof.roof, d, &work)
	if err != nil {
		return empty, zero, err
	}
	level, err := geom.RepresentedExactScalar(floor)
	if err != nil {
		return empty, zero, err
	}
	plane := mixedCoordinatePlane(2, level)
	remaining, err := roofBoxConstructionOptions(&work)
	if err != nil {
		return empty, zero, err
	}
	gap, err := geom.CertifySurfaceSupportingPlane(ctx, raw, plane[0], plane[1], plane[2], geom.SurfaceSupportingPlaneOptions{MaxOperations: remaining.MaxOperations, MaxCoefficientBits: remaining.MaxCoefficientBits})
	work.operations += gap.ExactOperations
	if err != nil {
		return empty, zero, err
	}
	if !gap.Complete {
		return empty, zero, geom.ErrConvergence
	}
	if gap.Coincident || gap.HalfspaceSign < 0 {
		history.ExactOperations = work.operations
		return empty, history, nil
	}
	if gap.HalfspaceSign != 1 || gap.SideMask != 0 || gap.CornerMask != 0 {
		return empty, zero, geom.ErrUnrepresentable
	}
	parts := make([]SewFace, 0, 6)
	add := func(patch *geom.SurfacePatch, reversed bool, owner MixedRoofFaceOwner) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		surface, err := geom.NewExactBezierSurface(patch)
		if err != nil {
			return err
		}
		opt, err := roofBoxConstructionOptions(&work)
		if err != nil {
			return err
		}
		part, err := ExactSplineFaceRectangle(ctx, surface, geom.ParameterRectangle{}, reversed, opt)
		if err != nil {
			return err
		}
		parts = append(parts, part)
		history.Faces = append(history.Faces, owner)
		return nil
	}
	if err := add(raw, false, MixedRoofFaceOwner{roofIndex, roof.roofFace}); err != nil {
		return empty, zero, err
	}
	bottom, err := roofBoxConstructionPatch(ctx, d, 1, 1, [][4]*big.Rat{
		{xy[0].Lower, xy[1].Lower, floor, big.NewRat(1, 1)}, {xy[0].Lower, xy[1].Upper, floor, big.NewRat(1, 1)},
		{xy[0].Upper, xy[1].Lower, floor, big.NewRat(1, 1)}, {xy[0].Upper, xy[1].Upper, floor, big.NewRat(1, 1)},
	}, &work)
	if err != nil {
		return empty, zero, err
	}
	if err := add(bottom, true, MixedRoofFaceOwner{floorIndex, q.box[floorIndex].floorFace}); err != nil {
		return empty, zero, err
	}
	data, err := raw.ExportExactBezierRecipe()
	if err != nil {
		return empty, zero, err
	}
	for boundary, fixed := range [4]float64{d.UMin, d.UMax, d.VMin, d.VMax} {
		axis, end := boundary/2, boundary%2
		varying := xy[1-axis]
		lo, _ := varying.Lower.Float64()
		hi, _ := varying.Upper.Float64()
		degree := [2]int{data.DegreeV, data.DegreeU}[axis]
		var controls [][4]*big.Rat
		for k := 0; k <= degree; k++ {
			i, j := data.DegreeU*end, k
			if axis == 1 {
				i, j = k, data.DegreeV*end
			}
			var top [4]*big.Rat
			for l, s := range data.Controls[i*(data.DegreeV+1)+j] {
				top[l], _ = new(big.Rat).SetString(s) // canonical exact recipe generated above
			}
			z, err := work.rat(new(big.Rat).Mul(top[3], floor))
			if err != nil {
				return empty, zero, err
			}
			controls = append(controls, [4]*big.Rat{top[0], top[1], z, top[3]}, top)
		}
		p, err := roofBoxConstructionPatch(ctx, geom.ParameterRectangle{UMin: lo, UMax: hi, VMax: 1}, degree, 1, controls, &work)
		if err != nil {
			return empty, zero, err
		}
		owner := 0
		if fixed != [4]float64{left.domain.UMin, left.domain.UMax, left.domain.VMin, left.domain.VMax}[boundary] {
			owner = 1
		}
		if err := add(p, axis == end, MixedRoofFaceOwner{owner, q.box[owner].sideFaces[boundary]}); err != nil {
			return empty, zero, err
		}
	}
	remaining, err = roofBoxConstructionOptions(&work)
	if err != nil {
		return empty, zero, err
	}
	out, _, err := Sew(ctx, parts, SewOptions{Tolerance: max(a.tolerance, b.tolerance), Proof: edgecheck.Options{MaxWork: remaining.MaxOperations, MaxBits: remaining.MaxCoefficientBits}})
	if err != nil {
		return empty, zero, err
	}
	if err := ctx.Err(); err != nil {
		return empty, zero, err
	}
	if err := roofBoxCheckSourceEdges(ctx, out, &work); err != nil {
		return empty, zero, err
	}
	history.ExactOperations = work.operations
	return out, history, nil
}

func roofBoxCheckSourceEdges(ctx context.Context, s Solid, w *mixedRoofLineWork) error {
	left := w.o.MaxOperations - w.operations
	if left < 1 {
		return geom.ErrConvergence
	}
	check, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: left, MaxBits: w.o.MaxCoefficientBits})
	if err != nil {
		return err
	}
	if !check.Complete {
		return geom.ErrUnrepresentable
	}
	for _, use := range check.Uses {
		if use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0 {
			return geom.ErrUnrepresentable
		}
	}
	return nil
}

func roofBoxConstructionOptions(w *mixedRoofLineWork) (geom.SurfacePatchOptions, error) {
	n := w.o.MaxOperations - w.operations
	if n < 1 {
		return geom.SurfacePatchOptions{}, geom.ErrConvergence
	}
	return geom.SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: w.o.MaxCoefficientBits}, nil
}

func roofBoxRestrictedPatch(p *geom.SurfacePatch, d geom.ParameterRectangle, w *mixedRoofLineWork) (*geom.SurfacePatch, error) {
	options, err := roofBoxConstructionOptions(w)
	if err != nil {
		return nil, err
	}
	out, work, err := p.TrimWithWorkLimit(d, options.MaxOperations)
	w.operations += work
	return out, err
}

func roofBoxConstructionPatch(ctx context.Context, d geom.ParameterRectangle, du, dv int, net [][4]*big.Rat, w *mixedRoofLineWork) (*geom.SurfacePatch, error) {
	o, err := roofBoxConstructionOptions(w)
	if err != nil {
		return nil, err
	}
	p, err := geom.NewExactHomogeneousBezierSurface(ctx, du, dv, net, o)
	if err != nil {
		return nil, err
	}
	w.operations += p.ExactOperations()
	o, err = roofBoxConstructionOptions(w)
	if err != nil {
		return nil, err
	}
	p, err = p.Reparameterize(d, false, false, o)
	if err != nil {
		return nil, err
	}
	w.operations += p.ExactOperations()
	return p, nil
}

// Weak one-sign Bernstein controls prove roof order on each closed subcell,
// even if the roofs are tangent there. Fit independent exact halvings of each
// axis. The small fixed grid is sufficient, so mixed-sign controls fail closed.
func proveRoofBoxPowerOrder(ctx context.Context, p roofPowerGrid, w *mixedRoofLineWork) (int, error) {
	z, m, h := new(big.Rat), big.NewRat(1, 2), big.NewRat(1, 1)
	chart := make([][][4]*big.Rat, len(p))
	for i, row := range p {
		chart[i] = make([][4]*big.Rat, len(row))
		for j, c := range row {
			chart[i][j] = [4]*big.Rat{z, z, c, z}
		}
	}
	du, dv := len(p)-1, len(p[0])-1
	binomial := func(k, n int) *big.Int { return new(big.Int).Binomial(int64(n), int64(k)) }
	factor := func(index, power, degree int) *big.Rat {
		return new(big.Rat).SetFrac(binomial(power, index), binomial(power, degree))
	}
	order := 0
	for i := range 2 {
		for j := range 2 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			cuts := [3]*big.Rat{z, m, h}
			cell, err := mixedRoofRebasePower(chart, geom.ParameterRectangle{UMax: 1, VMax: 1}, [2][2]*big.Rat{{cuts[i], cuts[i+1]}, {cuts[j], cuts[j+1]}}, w)
			if err != nil {
				return 0, err
			}
			for a := 0; a <= du; a++ {
				for b := 0; b <= dv; b++ {
					s := new(big.Rat)
					for u := 0; u <= a; u++ {
						for v := 0; v <= b; v++ {
							c := cell[u][v][2]
							if c.Sign() == 0 {
								continue
							}
							x, err := w.rat(new(big.Rat).Mul(c, factor(a, u, du)))
							if err != nil {
								return 0, err
							}
							if _, err := w.rat(x.Mul(x, factor(b, v, dv))); err != nil {
								return 0, err
							}
							if _, err := w.rat(s.Add(s, x)); err != nil {
								return 0, err
							}
						}
					}
					if s.Sign() != 0 {
						if order != 0 && order != s.Sign() {
							return 0, nil
						}
						order = s.Sign()
					}
				}
			}
		}
	}
	return order, nil
}
