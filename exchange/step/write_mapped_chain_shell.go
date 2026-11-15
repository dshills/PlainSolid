package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"slices"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// MappedChainShellSTEPOptions bounds exact source geometry and selects the
// positive tolerance for ordinary native Solid construction. Zero tolerance
// selects 1e-9. It does not relax the continuous curve equalities.
type MappedChainShellSTEPOptions struct {
	Geometry  geom.SurfacePatchOptions
	Tolerance float64
	// AllowMultiSpanRoofs admits one interior original support knot per roof.
	// Every emitted or read coedge must fit a closed original knot rectangle.
	AllowMultiSpanRoofs bool
	// AllowTwoInteriorRoofKnots admits up to two interior support knots per
	// roof. Also requires AllowMultiSpanRoofs.
	AllowTwoInteriorRoofKnots bool
	// AllowUnsplitSupportKnot enables one foreign shared source EDGE_CURVE
	// with a piecewise model spline and both original UV trims crossing the
	// same support knot stations. Allows one crossing, or two with
	// AllowTwoInteriorRoofKnots. Also requires AllowMultiSpanRoofs.
	AllowUnsplitSupportKnot bool
	// AllowAsynchronousSupportKnots admits two distinct unsplit foreign
	// shared edges, each with a single model spline and one original owner
	// changing support cells. Requires AllowUnsplitSupportKnot.
	AllowAsynchronousSupportKnots bool
	// AllowMixedSupportKnots admits one original foreign shared edge with a
	// stored model knot crossed by both owners and one additional single-owner
	// crossing. Requires AllowUnsplitSupportKnot and AllowTwoInteriorRoofKnots.
	AllowMixedSupportKnots bool
	// AllowThreeSupportKnots admits one original foreign edge with a stored
	// model knot crossed by both owners and one separate cut per owner.
	// Requires AllowUnsplitSupportKnot and AllowTwoInteriorRoofKnots.
	AllowThreeSupportKnots bool
	// AllowIndependentOwnerBases admits an explicit coedge graph with a whole
	// original spline for each owner of every shared edge, on both roofs. The
	// model stations use a common integer scale of at most 53 bits in STEP.
	AllowIndependentOwnerBases bool
}

// WriteMappedChainRectangularShell writes a single independently certified
// rectangular source shell using standard explicit spline surfaces, model
// curves and paired pcurves. Each mapped coedge retains its whole original
// spline basis under a parameter-valued STEP trim. No model sampling or
// polynomial approximation supplies the exact Bézier edge coefficients.
func WriteMappedChainRectangularShell(ctx context.Context, w io.Writer, shell trimmed.MappedSplineChainShell, name string, unit float64, l Limits, o MappedChainShellSTEPOptions) error {
	if ctx == nil || w == nil || !utf8.ValidString(name) || !validLengthUnit(unit) {
		return geom.ErrInvalid
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-9
	}
	bases := shell.IndependentOwnerBases()
	independent := bases == [2]bool{true, true}
	if bases[0] != bases[1] || (independent && !o.AllowIndependentOwnerBases) {
		return fmt.Errorf("mapped STEP owner basis arrangement: %w", geom.ErrConvergence)
	}
	native, err := shell.BuildRectangularSolid(ctx, o.Tolerance, o.Geometry)
	if err != nil {
		return err
	}
	raw, err := encodeMappedChainRectangularShell(ctx, native, name, unit, independent, o.Geometry)
	if err != nil {
		return err
	}
	if _, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), l, o); err != nil {
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

type mappedChainStepOwner struct {
	face int
	use  trimmed.Use
}

func encodeMappedChainRectangularShell(ctx context.Context, native trimmed.MappedChainRectangularSolid, name string, unit float64, independent bool, o geom.SurfacePatchOptions) ([]byte, error) {
	faces, edges, vertices := native.Solid.Faces(), native.Solid.Edges(), native.Solid.Vertices()
	stations := native.ChainStations
	if independent {
		var err error
		stations, err = mappedChainSTEPIntegerStations(stations)
		if err != nil {
			return nil, err
		}
	}
	mapped := make(map[int][2]*big.Rat, len(native.ChainEdges))
	for i, id := range native.ChainEdges {
		mapped[id] = [2]*big.Rat{stations[i], stations[i+1]}
	}
	uses := make([][]mappedChainStepOwner, len(edges))
	for i, face := range faces {
		for _, use := range face.Outer {
			uses[use.Edge] = append(uses[use.Edge], mappedChainStepOwner{i, use})
		}
	}
	var out bytes.Buffer
	var stageErr error
	err := writeStandalone(&out, unit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		var root int64
		root, stageErr = emitMappedChainRectangularShell(ctx, emit, name, faces, vertices, uses, native, mapped, independent, o)
		if stageErr != nil {
			return nil
		}
		return []int64{root}
	})
	if err != nil {
		return nil, err
	}
	if stageErr != nil {
		return nil, stageErr
	}
	return out.Bytes(), nil
}

// An affine change to the model knot domain leaves each exact model net and
// both independently parameterized pcurves unchanged. A common integer scale
// retains stations such as 1/6 without a rounded STEP real.
func mappedChainSTEPIntegerStations(stations []*big.Rat) ([]*big.Rat, error) {
	scale := big.NewInt(1)
	for _, station := range stations {
		d := station.Denom()
		gcd := new(big.Int).GCD(nil, nil, scale, d)
		scale.Mul(scale.Quo(scale, gcd), d)
		if scale.BitLen() > 53 {
			return nil, geom.ErrUnrepresentable
		}
	}
	factor := new(big.Rat).SetInt(scale)
	out := make([]*big.Rat, len(stations))
	for i, station := range stations {
		out[i] = new(big.Rat).Mul(station, factor)
	}
	return out, nil
}

func emitMappedChainRectangularShell(ctx context.Context, emit func(string) int64, name string, faces []trimmed.Face, vertices []geom.Vec3, uses [][]mappedChainStepOwner, native trimmed.MappedChainRectangularSolid, mapped map[int][2]*big.Rat, independent bool, o geom.SurfacePatchOptions) (int64, error) {
	surfaces := make([]int64, len(faces))
	for i, face := range faces {
		p, ok := face.Surface.(*geom.BSplineSurface)
		if !ok {
			return 0, geom.ErrConvergence
		}
		var err error
		surfaces[i], err = emitMappedChainStepSurface(emit, p)
		if err != nil {
			return 0, err
		}
	}
	vertexIDs := make([]int64, len(vertices))
	for i, point := range vertices {
		p, err := mappedChainStepDecimals(mappedChainStepFloats([]float64{point.X, point.Y, point.Z}))
		if err != nil {
			return 0, err
		}
		id := emit("CARTESIAN_POINT(''," + p + ")")
		vertexIDs[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", id))
	}
	uvContext := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
	edgeIDs := make([]int64, len(uses))
	edges := native.Solid.Edges()
	bases := make([]mappedChainStepBasis, len(faces))
	for i, owners := range uses {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if len(owners) != 2 {
			return 0, geom.ErrConvergence
		}
		a := owners[0]
		model, err := trimmed.ExactTrimModelBezier(ctx, faces[a.face].Surface, a.use.Curve, a.use.Reversed, o)
		if err != nil {
			return 0, fmt.Errorf("mapped STEP edge %d: %w", i, err)
		}
		net, _, err := model.BezierControlNet(ctx, o)
		if err != nil {
			return 0, err
		}
		r, chain := mapped[i]
		if !chain {
			r = [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
		}
		modelID, err := mappedChainStepSplineFromBezier(net.Control, 3, r[0], r[1]).emit(emit)
		if err != nil {
			return 0, fmt.Errorf("mapped STEP model edge %d: %w", i, err)
		}
		pc := make([]int64, 2)
		for j, owner := range owners {
			basis := &bases[owner.face]
			if independent {
				basis = &mappedChainStepBasis{}
			}
			curve, err := emitMappedChainStepUse(emit, owner.use, chain, basis)
			if err != nil {
				return 0, fmt.Errorf("mapped STEP owner %d edge %d: %w", owner.face, i, err)
			}
			rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", curve, uvContext))
			pc[j] = emit(fmt.Sprintf("PCURVE('',#%d,#%d)", surfaces[owner.face], rep))
		}
		sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,%s,.CURVE_3D.)", modelID, references(pc)))
		e := edges[i]
		edgeIDs[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertexIDs[e.Start], vertexIDs[e.End], sc))
	}
	faceIDs := make([]int64, len(faces))
	for i, face := range faces {
		wire := make([]int64, len(face.Outer))
		for j, use := range face.Outer {
			wire[j] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edgeIDs[use.Edge], mappedChainStepSense(!use.Reversed)))
		}
		loop := emit("EDGE_LOOP(''," + references(wire) + ")")
		side := mappedChainStepSense(!face.Reversed)
		bound := emit(fmt.Sprintf("FACE_OUTER_BOUND('',#%d,%s)", loop, side))
		faceIDs[i] = emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,%s)", bound, surfaces[i], side))
	}
	closed := emit("CLOSED_SHELL(''," + references(faceIDs) + ")")
	return emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(name), closed)), nil
}

type mappedChainStepBasis struct {
	id     int64
	source *geom.BSpline
}

func emitMappedChainStepUse(emit func(string) int64, use trimmed.Use, chain bool, basis *mappedChainStepBasis) (int64, error) {
	if chain {
		p, ok := use.Curve.(trimmed.MappedSplineUV)
		if !ok {
			return 0, geom.ErrConvergence
		}
		source := p.Source()
		if basis.id == 0 {
			id, err := mappedChainStepSplineFromSource(source.Curve).emit(emit)
			if err != nil {
				return 0, err
			}
			*basis = mappedChainStepBasis{id: id, source: source.Curve}
		} else if prior := basis.source; prior != source.Curve &&
			(prior.Degree() != source.Curve.Degree() ||
				!slices.Equal(prior.Poles(), source.Curve.Poles()) ||
				!slices.Equal(prior.Weights(), source.Curve.Weights()) ||
				!slices.Equal(prior.Knots(), source.Curve.Knots())) {
			return 0, fmt.Errorf("owner coedges require one common original spline basis: %w", geom.ErrConvergence)
		}
		lo, err := mappedChainStepDecimal(mappedChainStepRat(source.Range.Min))
		if err != nil {
			return 0, err
		}
		hi, err := mappedChainStepDecimal(mappedChainStepRat(source.Range.Max))
		if err != nil {
			return 0, err
		}
		sense := mappedChainStepSense(source.Reversed == use.Reversed)
		return emit(fmt.Sprintf("TRIMMED_CURVE('',#%d,(PARAMETER_VALUE(%s)),(PARAMETER_VALUE(%s)),%s,.PARAMETER.)", basis.id, lo, hi, sense)), nil
	}
	var source [][3]*big.Rat
	switch c := use.Curve.(type) {
	case trimmed.ExactQuadraticUV:
		h := c.HomogeneousControls()
		source = h[:]
	case trimmed.ExactBezierUV:
		source = c.HomogeneousControls()
	default:
		return 0, geom.ErrConvergence
	}
	if use.Reversed {
		slices.Reverse(source)
	}
	h := make([][4]*big.Rat, len(source))
	for i, p := range source {
		h[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), p[2]}
	}
	return mappedChainStepSplineFromBezier(h, 2, new(big.Rat), big.NewRat(1, 1)).emit(emit)
}

func mappedChainStepSense(forward bool) string {
	if forward {
		return ".T."
	}
	return ".F."
}
