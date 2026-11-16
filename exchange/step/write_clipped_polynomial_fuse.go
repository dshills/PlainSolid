package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"unicode/utf8"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ClippedPolynomialFuseSTEPOptions controls native coedge reproof and the
// representation of derived model and UV poles in standard STEP decimals.
// PoleTolerance zero requires exact decimals. A positive value bounds each
// Euclidean pole coordinate error in its own chart; preserved exact weights
// give the same uniform coordinate bound on the corresponding curve.
type ClippedPolynomialFuseSTEPOptions struct {
	Proof         edgecheck.Options
	PoleTolerance float64
}

// WriteClippedPolynomialFuse transfers a source-proved stepped solid with
// its analytic supports, native model edges and both incident pcurves. The
// tessellated display is not used. LengthUnit gives metres per model unit.
func WriteClippedPolynomialFuse(ctx context.Context, w io.Writer, fuse *trimmed.ClippedPolynomialFuse, name string, lengthUnit float64, l Limits, o ClippedPolynomialFuseSTEPOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, fuse.Boundary(), name, lengthUnit, l, o)
}

func writeExactBezierSingleSTEP(ctx context.Context, w io.Writer, s trimmed.Solid, name string, lengthUnit float64, l Limits, o ClippedPolynomialFuseSTEPOptions) error {
	return writeExactBezierSingleSTEPMode(ctx, w, s, name, lengthUnit, l, o, false)
}

func writeExactBezierSingleSTEPMode(ctx context.Context, w io.Writer, s trimmed.Solid, name string, lengthUnit float64, l Limits, o ClippedPolynomialFuseSTEPOptions, splitKnots bool) error {
	return writeExactBezierSingleSTEPWithBoundary(ctx, w, s, name, lengthUnit, l, o, splitKnots, nil)
}

func writeExactBezierSingleSTEPWithBoundary(ctx context.Context, w io.Writer, s trimmed.Solid, name string, lengthUnit float64, l Limits, o ClippedPolynomialFuseSTEPOptions, splitKnots bool, prepare func(context.Context, trimmed.ExactBezierBoundary, edgecheck.Options) error) error {
	if ctx == nil || w == nil || !utf8.ValidString(name) || len(name) > 1024 || !validLengthUnit(lengthUnit) ||
		!finiteNumber(o.PoleTolerance) || o.PoleTolerance < 0 {
		return geom.ErrInvalid
	}
	l, err := l.defaults()
	if err != nil {
		return err
	}
	if len(s.Topology().Shells) != 1 || len(s.Vertices()) > l.MaxVertices || len(s.Faces()) > l.MaxFaces || len(s.Edges()) > l.MaxEdgeUses/2 {
		return geom.ErrConvergence
	}
	extract := s.ExtractExactBezierBoundary
	if splitKnots {
		extract = s.ExtractExactBezierBoundaryAtRationalKnots
	}
	b, err := extract(ctx, o.Proof)
	if err != nil {
		return err
	}
	if len(b.Vertices) > l.MaxVertices || len(b.Edges) > l.MaxEdgeUses/2 {
		return geom.ErrConvergence
	}
	if prepare != nil && o.PoleTolerance == 0 {
		if err := prepare(ctx, b, o.Proof); err != nil {
			return err
		}
	}
	staged := clippedFuseSTEPStage{limit: l.MaxBytes}
	var geometryErr error
	err = writeStandalone(&staged, lengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		var id int64
		id, geometryErr = emitExactBezierSTEPBody(ctx, emit, b, name, mappedChainStepRat(o.PoleTolerance))
		return []int64{id}
	})
	if geometryErr != nil {
		return geometryErr
	}
	if err != nil {
		return err
	}
	if _, err := Parse(bytes.NewReader(staged.Bytes()), l); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(staged.Bytes())
	if err == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return err
}

// VerifyClippedPolynomialFuseSTEP authenticates the canonical STEP DATA
// graph against a source-proved native Fuse, including its two pcurves per
// edge, orientations, name and units. It accepts harmless header and layout
// changes. It does not import arbitrary independent STEP or retessellate the
// material. Exact native geometry remains in the source-backed solid.
func VerifyClippedPolynomialFuseSTEP(ctx context.Context, input io.Reader, fuse *trimmed.ClippedPolynomialFuse, name string, lengthUnit float64, l Limits, o ClippedPolynomialFuseSTEPOptions) error {
	if ctx == nil || input == nil || fuse == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actual, err := Parse(input, l)
	if err != nil {
		return err
	}
	var canonical bytes.Buffer
	if err := WriteClippedPolynomialFuse(ctx, &canonical, fuse, name, lengthUnit, l, o); err != nil {
		return err
	}
	want, err := Parse(&canonical, l)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual.Entities, want.Entities) {
		return fmt.Errorf("step: clipped Fuse DATA differs from its source proof: %w", geom.ErrConvergence)
	}
	return ctx.Err()
}

type clippedFuseSTEPStage struct {
	bytes.Buffer
	limit int64
}

func (s *clippedFuseSTEPStage) Write(p []byte) (int, error) {
	if int64(len(p)) > s.limit-int64(s.Len()) {
		return 0, fmt.Errorf("step: analytic clipped output exceeds byte limit: %w", geom.ErrConvergence)
	}
	return s.Buffer.Write(p)
}

type clippedFuseSTEPOwner struct {
	face int
	uv   geom.ExactBezierSpan
}

func emitExactBezierSTEPBody(ctx context.Context, emit func(string) int64, b trimmed.ExactBezierBoundary, name string, poleTolerance *big.Rat) (int64, error) {
	supports := make([]int64, len(b.Faces))
	owners := make([][]clippedFuseSTEPOwner, len(b.Edges))
	for i, f := range b.Faces {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		surface, err := f.BSplineSupport()
		if err != nil {
			return 0, fmt.Errorf("step: exact Bézier support %d: %w", i, err)
		}
		supports[i], err = emitMappedChainStepSurface(emit, surface)
		if err != nil {
			return 0, err
		}
		for _, loop := range f.Loops {
			for _, u := range loop {
				owners[u.Edge] = append(owners[u.Edge], clippedFuseSTEPOwner{face: i, uv: u.UV})
			}
		}
	}
	vertices := make([]int64, len(b.Vertices))
	for i, p := range b.Vertices {
		coords, err := mappedChainStepDecimals(mappedChainStepFloats([]float64{p.X, p.Y, p.Z}))
		if err != nil {
			return 0, err
		}
		vertices[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", emit("CARTESIAN_POINT('',"+coords+")")))
	}
	uvContext := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
	edges := make([]int64, len(b.Edges))
	for i, e := range b.Edges {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if len(owners[i]) != 2 || owners[i][0].face == owners[i][1].face {
			return 0, geom.ErrConvergence
		}
		model, err := emitClippedFuseSTEPBezier(emit, e.Model, 3, poleTolerance)
		if err != nil {
			return 0, fmt.Errorf("step: exact Bézier model edge %d: %w", i, err)
		}
		var pc [2]int64
		for j, owner := range owners[i] {
			uv, err := emitClippedFuseSTEPBezier(emit, owner.uv, 2, poleTolerance)
			if err != nil {
				return 0, fmt.Errorf("step: exact Bézier edge %d face %d pcurve: %w", i, owner.face, err)
			}
			rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", uv, uvContext))
			pc[j] = emit(fmt.Sprintf("PCURVE('',#%d,#%d)", supports[owner.face], rep))
		}
		sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,%s,.CURVE_3D.)", model, references(pc[:])))
		edges[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[e.Start], vertices[e.End], sc))
	}
	faces := make([]int64, len(b.Faces))
	for i, f := range b.Faces {
		side := mappedChainStepSense(!f.Reversed)
		bounds := make([]int64, len(f.Loops))
		for j, loop := range f.Loops {
			wire := make([]int64, len(loop))
			for k, u := range loop {
				wire[k] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edges[u.Edge], mappedChainStepSense(!u.Reversed)))
			}
			loopID := emit("EDGE_LOOP(''," + references(wire) + ")")
			kind := "FACE_OUTER_BOUND"
			if j > 0 {
				kind = "FACE_BOUND"
			}
			bounds[j] = emit(fmt.Sprintf("%s('',#%d,%s)", kind, loopID, side))
		}
		faces[i] = emit(fmt.Sprintf("ADVANCED_FACE('',%s,#%d,%s)", references(bounds), supports[i], side))
	}
	shell := emit("CLOSED_SHELL(''," + references(faces) + ")")
	return emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(name), shell)), nil
}

func emitClippedFuseSTEPBezier(emit func(string) int64, span geom.ExactBezierSpan, dim int, tolerance *big.Rat) (int64, error) {
	spline := mappedChainStepSplineFromBezier(span.Control, dim, new(big.Rat), big.NewRat(1, 1))
	for i, p := range spline.poles {
		for j, q := range p {
			if _, err := mappedChainStepDecimal(q); err == nil {
				continue
			}
			// Topological endpoints must always retain their exact coordinates.
			if i == 0 || i == len(spline.poles)-1 || tolerance.Sign() == 0 {
				return 0, fmt.Errorf("pole coordinate has no terminating STEP decimal: %w", geom.ErrUnrepresentable)
			}
			x, _ := q.Float64()
			if !finiteNumber(x) {
				return 0, geom.ErrUnrepresentable
			}
			r := mappedChainStepRat(x)
			if new(big.Rat).Abs(new(big.Rat).Sub(q, r)).Cmp(tolerance) > 0 {
				return 0, geom.ErrUnrepresentable
			}
			p[j] = r
		}
	}
	return spline.emit(emit)
}
