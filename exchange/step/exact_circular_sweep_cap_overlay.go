package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/circularsweep"
	"plainsolid/geom"
)

const exactCircularSweepCapOverlayKind = "exact-circular-sweep-cap-overlay"

// ExactCircularSweepCapOverlay describes two original exact sweeps, their
// common signed-axis placement and their length unit in metres. The sweeps
// must share a complete cap. The STEP graph stores their common subdivision.
type ExactCircularSweepCapOverlay struct {
	Name       string
	Frame      geom.Frame
	Sweeps     [2]trimmed.ExactCircularSweep
	LengthUnit float64
}

// ExactCircularSweepCapOverlayProduct authenticates the source STEP graph
// against two supplied original profiles. Faces maps each original face to
// its STEP fragments. A removed cap maps to an empty slice.
type ExactCircularSweepCapOverlayProduct struct {
	Native      trimmed.CircularSweepCapOverlay
	Source      *File
	Body, Shell int64
	Faces       [2][][]int64
}

func buildExactCircularSweepCapOverlay(ctx context.Context, d ExactCircularSweepCapOverlay) (circularsweep.Overlay, error) {
	if !utf8.ValidString(d.Name) || len(d.Name) > 4096 || !planarSplineRigid(frameTransform(d.Frame)) {
		return circularsweep.Overlay{}, geom.ErrInvalid
	}
	return circularsweep.BuildCapOverlay(ctx, d.Sweeps[0], d.Sweeps[1])
}

func writeExactCircularSweepCapOverlayBoundary(ctx context.Context, w io.Writer, d ExactCircularSweepCapOverlay, p circularsweep.Overlay) error {
	supports := make([]circularBoundarySupport, len(p.Supports))
	for i, s := range p.Supports {
		supports[i] = circularBoundarySupport{capZ: s.Z, wall: s.Wall, segment: s.Segment}
	}
	return writeCircularBoundaryGraph(ctx, w, d.Name, d.Frame, d.LengthUnit, exactCircularSweepCapOverlayKind, p.Native.Solid, supports, p.Rims)
}

// WriteExactCircularSweepCapOverlay emits one standard analytic STEP B-rep
// with separate wall faces and a single shared rim at the joined cap. It
// exports the material boundary; the native overlay receipt keeps the
// unrefined source partitions and exact parameter maps.
func WriteExactCircularSweepCapOverlay(ctx context.Context, w io.Writer, d ExactCircularSweepCapOverlay) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, err := buildExactCircularSweepCapOverlay(ctx, d)
	if err != nil {
		return err
	}
	return writeExactCircularSweepCapOverlayBoundary(ctx, w, d, p)
}

// ReadExactCircularSweepCapOverlay accepts the canonical analytic STEP
// dialect and requires the two original sweeps. Their partitions cannot be
// inferred uniquely from the refined STEP solid. The entire supplied graph
// must match the independently reconstructed overlay.
func ReadExactCircularSweepCapOverlay(ctx context.Context, r io.Reader, d ExactCircularSweepCapOverlay, l Limits) (ExactCircularSweepCapOverlayProduct, error) {
	var zero ExactCircularSweepCapOverlayProduct
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := occurrenceColorSourceLimits(l)
	if err != nil {
		return zero, err
	}
	f, _, _, bodies, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil {
		return zero, err
	}
	if len(bodies) != 1 {
		return zero, geom.ErrDomain
	}
	p, err := buildExactCircularSweepCapOverlay(ctx, d)
	if err != nil {
		return zero, err
	}
	var expected bytes.Buffer
	if err = writeExactCircularSweepCapOverlayBoundary(ctx, &expected, d, p); err != nil {
		return zero, err
	}
	canonical, err := Parse(&expected, l)
	if err != nil {
		return zero, err
	}
	if err = sideDiskMatchWholeGraph(f, canonical); err != nil {
		return zero, fmt.Errorf("step: circular cap overlay source: %w", err)
	}
	body, err := simple(f, bodies[0], "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return zero, err
	}
	shellID := body.Args[1].Ref
	shell, err := simple(f, shellID, "CLOSED_SHELL", 2)
	if err != nil {
		return zero, err
	}
	fids, err := listRefs(shell.Args[1], len(p.Supports))
	if err != nil || len(fids) != len(p.Supports) {
		return zero, geom.ErrDomain
	}
	out := ExactCircularSweepCapOverlayProduct{Native: p.Native, Source: f, Body: bodies[0], Shell: shellID}
	for i, source := range p.Native.Faces {
		out.Faces[i] = make([][]int64, len(source))
		for j, split := range source {
			for _, k := range split {
				out.Faces[i][j] = append(out.Faces[i][j], fids[k])
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
