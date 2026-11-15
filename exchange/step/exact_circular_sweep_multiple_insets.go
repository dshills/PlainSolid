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

const exactCircularSweepInsetsSTEPKind = "exact-circular-sweep-insets"

// ExactCircularSweepInsets describes a host and independent inset guests on
// one or both caps. LengthUnit is metres per STEP length unit.
type ExactCircularSweepInsets struct {
	Name       string
	Frame      geom.Frame
	Host       trimmed.ExactCircularSweep
	Guests     []trimmed.ExactCircularSweep
	LengthUnit float64
}

// ExactCircularSweepInsetsProduct gives the exact reconstructed native
// join and each original face's STEP fragments. Each guest's joining cap
// is absent, and its entry in GuestFaces is zero.
type ExactCircularSweepInsetsProduct struct {
	Native      trimmed.CircularSweepInsets
	Source      *File
	Body, Shell int64
	HostFaces   [][]int64
	GuestFaces  [][]int64
}

func buildExactCircularSweepInsets(ctx context.Context, d ExactCircularSweepInsets) (circularsweep.Insets, error) {
	if !utf8.ValidString(d.Name) || len(d.Name) > 4096 || !planarSplineRigid(frameTransform(d.Frame)) {
		return circularsweep.Insets{}, geom.ErrInvalid
	}
	return circularsweep.BuildInsets(ctx, d.Host, d.Guests)
}

func writeExactCircularSweepInsetsBoundary(ctx context.Context, w io.Writer, d ExactCircularSweepInsets, p circularsweep.Insets) error {
	supports := make([]circularBoundarySupport, len(p.Supports))
	for i, s := range p.Supports {
		supports[i] = circularBoundarySupport{capZ: s.Z, wall: s.Wall, segment: s.Segment}
	}
	return writeCircularBoundaryGraph(ctx, w, d.Name, d.Frame, d.LengthUnit, exactCircularSweepInsetsSTEPKind, p.Native.Solid, supports, p.Rims)
}

// WriteExactCircularSweepInsets writes one analytic STEP solid with the
// original outer caps, guest walls, and each independent exposed floor.
func WriteExactCircularSweepInsets(ctx context.Context, w io.Writer, d ExactCircularSweepInsets) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, err := buildExactCircularSweepInsets(ctx, d)
	if err != nil {
		return err
	}
	return writeExactCircularSweepInsetsBoundary(ctx, w, d, p)
}

// ReadExactCircularSweepInsets compares the full canonical STEP graph with
// the exact boundary reconstructed independently from the supplied sources.
func ReadExactCircularSweepInsets(ctx context.Context, r io.Reader, d ExactCircularSweepInsets, l Limits) (ExactCircularSweepInsetsProduct, error) {
	var zero ExactCircularSweepInsetsProduct
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
	p, err := buildExactCircularSweepInsets(ctx, d)
	if err != nil {
		return zero, err
	}
	var expected bytes.Buffer
	if err = writeExactCircularSweepInsetsBoundary(ctx, &expected, d, p); err != nil {
		return zero, err
	}
	canonical, err := Parse(&expected, l)
	if err != nil {
		return zero, err
	}
	if err = sideDiskMatchWholeGraph(f, canonical); err != nil {
		return zero, fmt.Errorf("step: circular multi-inset source: %w", err)
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
	n := p.Native
	out := ExactCircularSweepInsetsProduct{Native: n, Source: f, Body: bodies[0], Shell: shellID,
		HostFaces: make([][]int64, len(n.HostFaces)), GuestFaces: make([][]int64, len(n.Guests))}
	for i, source := range n.HostFaces {
		for _, j := range source {
			out.HostFaces[i] = append(out.HostFaces[i], fids[j])
		}
	}
	for i, g := range n.Guests {
		out.GuestFaces[i] = make([]int64, len(g.Faces))
		for j, k := range g.Faces {
			if k >= 0 {
				out.GuestFaces[i][j] = fids[k]
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
