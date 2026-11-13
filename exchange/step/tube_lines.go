package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"unicode/utf8"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// TubeLine proposes exact paired UV endpoints in the certified segment's
// traversal order. Entries are [uA,vA,uB,vB]; rationals are read, not modified.
// Endpoint samples from EvaluatePair are not an exactness certificate.
type TubeLine struct {
	Name       string
	Segment    intersect.TubeSegment
	Start, End [4]*big.Rat
}

type TubeLineModel struct {
	Lines      []TubeLine
	LengthUnit float64
}

// TubeLineOptions bounds the number of segments and each exact identity proof.
// Zero MaxSegments selects 256. Proof uses geom.TubeAffineLineOptions defaults.
type TubeLineOptions struct {
	MaxSegments int
	Proof       geom.TubeAffineLineOptions
}

// WriteTubeLines independently proves each entire certified tube segment is
// exactly the proposed affine model line, then writes losslessly representable
// degree-one spline geometry through WriteCurves. Traversal becomes t=[0,1].
// This is standalone curve exchange: it exports neither p-curves, source tube
// certificates nor trimmed-face/shell topology. It never infers exact curves
// from samples. Invalid/unproved/unrepresentable inputs fail before writing.
func WriteTubeLines(ctx context.Context, w io.Writer, m TubeLineModel, o TubeLineOptions) error {
	if ctx == nil || w == nil {
		return errors.New("step: missing tube line context/writer")
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 256
	}
	if o.MaxSegments < 1 || len(m.Lines) == 0 || len(m.Lines) > o.MaxSegments {
		return errors.New("step: tube line count exceeds limit")
	}
	if !finiteNumber(m.LengthUnit) || m.LengthUnit <= 0 {
		return errors.New("step: invalid tube line units")
	}
	curves := CurveModel{LengthUnit: m.LengthUnit}
	for i, line := range m.Lines {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !utf8.ValidString(line.Name) {
			return errors.New("step: invalid tube line name")
		}
		proof, err := line.Segment.CertifyAffineLineExact(ctx, line.Start, line.End, o.Proof)
		if err != nil {
			return fmt.Errorf("step: tube line %d: %w", i, err)
		}
		if !proof.Proven || proof.Curve == nil {
			return fmt.Errorf("step: tube line %d has no exact affine proof", i)
		}
		curve, err := proof.Curve.ToBSpline()
		if err != nil {
			return fmt.Errorf("step: tube line %d: %w", i, err)
		}
		poles := curve.Poles()
		if curve.Degree() != 1 || len(poles) != 2 || poles[0] == poles[1] {
			return errors.New("step: unexpected or degenerate tube line")
		}
		curves.Curves = append(curves.Curves, SplineCurve{Name: line.Name, Geometry: curve, Closed: "F", SelfIntersect: "F"})
	}
	var staged bytes.Buffer
	if err := WriteCurves(&staged, curves); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := io.Copy(w, &staged)
	return err
}
