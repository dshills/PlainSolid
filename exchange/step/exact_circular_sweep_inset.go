package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const exactCircularSweepInsetSTEPKind = "exact-circular-sweep-inset"

// ExactCircularSweepInset is the pair of original intersecting cap sweeps and
// their common signed-coordinate placement. The guest's footprint must be
// strictly within the host at the shared cap.
type ExactCircularSweepInset struct {
	Name        string
	Frame       geom.Frame
	Host, Guest trimmed.ExactCircularSweep
	LengthUnit  float64
}

// ExactCircularSweepInsetProduct preserves the original roles after strict
// source reproof. HostFaces and GuestFaces hold original STEP entity IDs. A
// zero GuestFaces entry identifies the guest cap consumed by the join.
type ExactCircularSweepInsetProduct struct {
	Descriptor  ExactCircularSweepInset
	Native      trimmed.CircularSweepInset
	Source      *File
	Body, Shell int64
	HostFaces   [][]int64
	GuestFaces  []int64
}

// WriteExactCircularSweepInset writes the joined standard analytic B-rep.
// Original guest rims bound the host cap cutout and every exposed floor. No
// mesh or approximate trim is used for the transferred model.
func WriteExactCircularSweepInset(ctx context.Context, w io.Writer, d ExactCircularSweepInset) error {
	if ctx == nil || w == nil || !utf8.ValidString(d.Name) || len(d.Name) > 4096 || !planarSplineRigid(frameTransform(d.Frame)) {
		return geom.ErrInvalid
	}
	joined, err := trimmed.JoinExactCircularSweepInset(ctx, d.Host, d.Guest, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		return err
	}
	return writeCircularSweepInsetBoundary(ctx, w, d, joined)
}

func writeCircularSweepInsetBoundary(ctx context.Context, w io.Writer, d ExactCircularSweepInset, joined trimmed.CircularSweepInset) error {
	faces, edges := joined.Solid.Faces(), joined.Solid.Edges()
	supports, circles := make([]circularBoundarySupport, len(faces)), make([]trimmed.ExactCircularLoopSegment, len(edges))
	for i, src := range []trimmed.ExactCircularSweep{d.Host, d.Guest} {
		segments := append([]trimmed.ExactCircularLoopSegment(nil), src.Outer...)
		for _, h := range src.Holes {
			segments = append(segments, h...)
		}
		n := len(segments)
		edgeOffset := 0
		if i > 0 {
			edgeOffset = joined.GuestEdgeOffset
		}
		for j, s := range segments {
			circles[edgeOffset+j], circles[edgeOffset+j+n] = s, s
		}
		for j := 0; j < n+2; j++ {
			var roles []int
			if i == 0 {
				roles = joined.HostFaces[j]
			} else if k := joined.GuestFaces[j]; k >= 0 {
				roles = []int{k}
			}
			var support circularBoundarySupport
			if j == 0 {
				support.capZ = src.Bottom
			} else if j == 1 {
				support.capZ = src.Top
			} else {
				support = circularBoundarySupport{wall: true, segment: segments[j-2]}
			}
			for _, k := range roles {
				supports[k] = support
			}
		}
	}
	return writeCircularBoundaryGraph(ctx, w, d.Name, d.Frame, d.LengthUnit, exactCircularSweepInsetSTEPKind, joined.Solid, supports, circles)
}

func circularSweepInsetPlane(f *File, faceID int64) (geom.Frame, [3]*big.Rat, error) {
	var zero [3]*big.Rat
	r, err := simple(f, faceID, "ADVANCED_FACE", 4)
	if err != nil {
		return geom.Frame{}, zero, err
	}
	p, err := simple(f, r.Args[2].Ref, "PLANE", 2)
	if err != nil {
		return geom.Frame{}, zero, err
	}
	_, normal, _, err := occtLoopFrame(f, p.Args[1].Ref)
	if err != nil {
		return geom.Frame{}, zero, err
	}
	frame, err := sideDiskReadFrame(f, p.Args[1].Ref)
	return frame, normal, err
}

func circularSweepInsetSourceFrame(f *File, fids []int64) (geom.Frame, [2]float64, error) {
	l, _, err := circularSweepInsetPlane(f, fids[0])
	if err != nil {
		return geom.Frame{}, [2]float64{}, err
	}
	h, _, err := circularSweepInsetPlane(f, fids[1])
	if err != nil || l.XAxis() != h.XAxis() || l.ZAxis() != h.ZAxis() {
		return geom.Frame{}, [2]float64{}, geom.ErrDomain
	}
	face, err := simple(f, fids[2], "ADVANCED_FACE", 4)
	if err != nil {
		return geom.Frame{}, [2]float64{}, err
	}
	surface := f.Entities[face.Args[2].Ref]
	if len(surface.Records) != 1 || len(surface.Records[0].Args) < 2 ||
		surface.Records[0].Type != "PLANE" && surface.Records[0].Type != "CYLINDRICAL_SURFACE" {
		return geom.Frame{}, [2]float64{}, geom.ErrDomain
	}
	wall, _, _, err := occtLoopFrame(f, surface.Records[0].Args[1].Ref)
	if err != nil {
		return geom.Frame{}, [2]float64{}, err
	}
	axis, p := occtLoopVec(l.ZAxis()), occtLoopVec(l.Origin())
	offset := occtLoopDot(axis, occtLoopSub(wall, p))
	for j := range p {
		p[j].Add(p[j], new(big.Rat).Mul(axis[j], offset))
	}
	origin, err := obliqueNURBSFloatPoint(p)
	if err != nil {
		return geom.Frame{}, [2]float64{}, err
	}
	frame, err := geom.NewFrame(origin, l.XAxis(), l.ZAxis())
	if err != nil {
		return geom.Frame{}, [2]float64{}, err
	}
	z0, err := sideDiskLocal(frame, l.Origin())
	if err != nil {
		return geom.Frame{}, [2]float64{}, err
	}
	z1, err := sideDiskLocal(frame, h.Origin())
	if err != nil || z0.X != 0 || z0.Y != 0 || z1.X != 0 || z1.Y != 0 || z0.Z >= z1.Z {
		return geom.Frame{}, [2]float64{}, geom.ErrDomain
	}
	return frame, [2]float64{z0.Z, z1.Z}, nil
}

func circularSweepInsetRings(f *File, id int64, frame geom.Frame, z float64) ([][]trimmed.ExactCircularLoopSegment, int, error) {
	uses, err := exactLoopRimSourceUses(f, id, 0, true)
	if err != nil {
		return nil, 0, err
	}
	out := make([][]trimmed.ExactCircularLoopSegment, len(uses))
	point := func(id int64) (geom.Vec3, error) {
		v, err := simple(f, id, "VERTEX_POINT", 2)
		if err != nil {
			return geom.Vec3{}, err
		}
		w, err := splitTriple(f, v.Args[1].Ref, "CARTESIAN_POINT")
		if err != nil {
			return geom.Vec3{}, err
		}
		return sideDiskLocal(frame, w)
	}
	total := 0
	for i, ring := range uses {
		for _, eid := range ring {
			e, err := simple(f, eid, "EDGE_CURVE", 5)
			if err != nil {
				return nil, 0, err
			}
			p, er := point(e.Args[1].Ref)
			q, ex := point(e.Args[2].Ref)
			if er != nil || ex != nil || p.Z != z || q.Z != z {
				return nil, 0, geom.ErrDomain
			}
			seg := trimmed.ExactCircularLoopSegment{Start: geom.Vec2{X: p.X, Y: p.Y}, End: geom.Vec2{X: q.X, Y: q.Y}}
			actual := f.Entities[e.Args[3].Ref]
			if len(actual.Records) != 1 {
				return nil, 0, geom.ErrDomain
			}
			c := actual.Records[0]
			switch {
			case c.Type == "LINE" && len(c.Args) == 3:
			case c.Type == "CIRCLE" && len(c.Args) == 3:
				cf, err := sideDiskReadFrame(f, c.Args[1].Ref)
				if err != nil {
					return nil, 0, err
				}
				a, err := sideDiskLocal(frame, cf.Origin())
				if err != nil || a.Z != z {
					return nil, 0, geom.ErrDomain
				}
				seg.Center = geom.Vec2{X: a.X, Y: a.Y}
				seg.Radius, err = cylinderSupportNumber(c.Args[2])
				if err != nil || seg.Radius <= 0 {
					return nil, 0, geom.ErrDomain
				}
			default:
				return nil, 0, geom.ErrDomain
			}
			out[i] = append(out[i], seg)
			total++
		}
	}
	return out, total, nil
}

// ReadExactCircularSweepInset accepts the canonical analytic writer dialect.
// The unconsumed caps recover both original profiles. Reconstructing the
// inset proves the complete wire ownership, floor faces and original supports
// by comparing every record in the rebuilt STEP graph.
func ReadExactCircularSweepInset(ctx context.Context, r io.Reader, l Limits) (ExactCircularSweepInsetProduct, error) {
	var zero ExactCircularSweepInsetProduct
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
	f, _, unit, bodies, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil {
		return zero, err
	}
	if len(bodies) != 1 {
		return zero, geom.ErrDomain
	}
	b, err := simple(f, bodies[0], "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return zero, err
	}
	name, err := textValue(b.Args[0])
	if err != nil || len(name) > 4096 {
		return zero, geom.ErrDomain
	}
	sh, err := simple(f, b.Args[1].Ref, "CLOSED_SHELL", 2)
	if err != nil {
		return zero, err
	}
	fids, err := listRefs(sh.Args[1], 9)
	if err != nil || len(fids) > 151 {
		return zero, geom.ErrDomain
	}
	frame, hostZ, err := circularSweepInsetSourceFrame(f, fids)
	if err != nil {
		return zero, err
	}
	axis := occtLoopVec(frame.ZAxis())
	guestCapIndex := 0
	var far float64
	for i := 2; i < len(fids); i++ {
		p, n, err := circularSweepInsetPlane(f, fids[i])
		if err != nil || !occtLoopParallel(n, axis) {
			continue
		}
		v, err := sideDiskLocal(frame, p.Origin())
		if err != nil || v.X != 0 || v.Y != 0 || p.XAxis() != frame.XAxis() || p.ZAxis() != frame.ZAxis() {
			return zero, geom.ErrDomain
		}
		guestCapIndex, far = i, v.Z
		break
	}
	if guestCapIndex < 5 || guestCapIndex > 66 || far >= hostZ[0] && far <= hostZ[1] {
		return zero, geom.ErrDomain
	}
	hostUnconsumed := 0
	guestZ := [2]float64{hostZ[1], far}
	if far < hostZ[0] {
		hostUnconsumed, guestZ = 1, [2]float64{far, hostZ[0]}
	}
	host, n, err := circularSweepInsetRings(f, fids[hostUnconsumed], frame, hostZ[hostUnconsumed])
	if err != nil || n+2 != guestCapIndex {
		return zero, geom.ErrDomain
	}
	guest, count, err := circularSweepInsetRings(f, fids[guestCapIndex], frame, far)
	if err != nil || len(fids) != 2+n+1+count+len(guest)-1 {
		return zero, geom.ErrDomain
	}
	d := ExactCircularSweepInset{
		Name: name, Frame: frame, LengthUnit: unit,
		Host:  trimmed.ExactCircularSweep{Outer: host[0], Holes: host[1:], Bottom: hostZ[0], Top: hostZ[1]},
		Guest: trimmed.ExactCircularSweep{Outer: guest[0], Holes: guest[1:], Bottom: guestZ[0], Top: guestZ[1]},
	}
	j, err := trimmed.JoinExactCircularSweepInset(ctx, d.Host, d.Guest, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		return zero, err
	}
	var expected bytes.Buffer
	if err = writeCircularSweepInsetBoundary(ctx, &expected, d, j); err != nil {
		return zero, err
	}
	canonical, err := Parse(&expected, l)
	if err != nil {
		return zero, err
	}
	if err = sideDiskMatchWholeGraph(f, canonical); err != nil {
		return zero, fmt.Errorf("step: exact circular sweep inset source: %w", err)
	}
	out := ExactCircularSweepInsetProduct{
		Descriptor: d, Native: j, Source: f, Body: bodies[0], Shell: b.Args[1].Ref,
		HostFaces: make([][]int64, len(j.HostFaces)), GuestFaces: make([]int64, len(j.GuestFaces)),
	}
	for i, fragments := range j.HostFaces {
		for _, k := range fragments {
			out.HostFaces[i] = append(out.HostFaces[i], fids[k])
		}
	}
	for i, k := range j.GuestFaces {
		if k >= 0 {
			out.GuestFaces[i] = fids[k]
		}
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
