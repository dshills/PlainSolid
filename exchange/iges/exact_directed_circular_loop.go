package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ExactDirectedCircularLoopExtrusion describes a positive model-Z sweep. The
// material outer wire is CCW and each disjoint hole is clockwise; individual
// circular arcs may turn either way. Source units are metres per length unit.
type ExactDirectedCircularLoopExtrusion struct {
	Outer             []trimmed.ExactCircularLoopSegment
	Holes             [][]trimmed.ExactCircularLoopSegment
	Bottom, Top, Unit float64
}

type directedIGESRoles struct {
	vertices, edges, solid, shell int
	rims                          [2][]int
	walls, verticals, ringOffsets []int
	caps                          [2]int
	capLoops                      [2][]int
	faceIDs, curveIDs             []int
}

func directedIGESFloat(x float64) (string, error) {
	if !finite(x) {
		return "", geom.ErrInvalid
	}
	r := new(big.Rat).SetFloat64(x)
	digits := r.Denom().BitLen() - 1
	if digits > 60 || r.Num().BitLen() > 192 {
		return "", geom.ErrUnrepresentable
	}
	s := r.FloatString(digits)
	if len(s) > 64 {
		return "", geom.ErrUnrepresentable
	}
	return s, nil
}

type directedIGESAuthor struct {
	topologyAuthor
	err error
}

func (a *directedIGESAuthor) addReal(kind, form int, v ...float64) int {
	s := make([]string, 1, len(v)+1)
	s[0] = strconv.Itoa(kind)
	for _, x := range v {
		t, e := directedIGESFloat(x)
		if e != nil {
			a.err = e
		}
		s = append(s, t)
	}
	return a.add(kind, form, strings.Join(s, ",")+";")
}

func directedIGESClockwise(s trimmed.ExactCircularLoopSegment) bool {
	x := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	ax := new(big.Rat).Sub(x(s.Start.X), x(s.Center.X))
	ay := new(big.Rat).Sub(x(s.Start.Y), x(s.Center.Y))
	bx := new(big.Rat).Sub(x(s.End.X), x(s.Center.X))
	by := new(big.Rat).Sub(x(s.End.Y), x(s.Center.Y))
	return ax.Mul(ax, by).Cmp(ay.Mul(ay, bx)) < 0
}

func buildDirectedIGES(ctx context.Context, d ExactDirectedCircularLoopExtrusion) (topologyAuthor, directedIGESRoles, trimmed.Solid, error) {
	var zero directedIGESRoles
	native, err := trimmed.ExtrudeExactDirectedCircularLoops(ctx, d.Outer, d.Holes, d.Bottom, d.Top, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		return topologyAuthor{}, zero, trimmed.Solid{}, err
	}
	flat := append([]trimmed.ExactCircularLoopSegment(nil), d.Outer...)
	offsets := []int{0, len(flat)}
	for _, ring := range d.Holes {
		flat = append(flat, ring...)
		offsets = append(offsets, len(flat))
	}
	n := len(flat)
	supports, circles := make([]directedIGESSupport, n+2), make([]trimmed.ExactCircularLoopSegment, 3*n)
	supports[0].z, supports[1].z = d.Bottom, d.Top
	for i, segment := range flat {
		supports[2+i] = directedIGESSupport{z: d.Bottom, wall: true, segment: segment}
		circles[i], circles[n+i] = segment, segment
	}
	a, roles, err := buildDirectedIGESBoundary(ctx, native, supports, circles)
	if err != nil {
		return topologyAuthor{}, zero, trimmed.Solid{}, err
	}
	roles.ringOffsets = offsets
	roles.rims = [2][]int{roles.curveIDs[:n], roles.curveIDs[n : 2*n]}
	roles.verticals = roles.curveIDs[2*n:]
	roles.caps = [2]int{roles.faceIDs[0], roles.faceIDs[1]}
	roles.walls = roles.faceIDs[2:]
	return a, roles, native, nil
}

type directedIGESSupport struct {
	z       float64
	wall    bool
	segment trimmed.ExactCircularLoopSegment
}

func buildDirectedIGESBoundary(ctx context.Context, native trimmed.Solid, supports []directedIGESSupport, circles []trimmed.ExactCircularLoopSegment) (topologyAuthor, directedIGESRoles, error) {
	var zero directedIGESRoles
	a := directedIGESAuthor{}
	role := directedIGESRoles{}
	verts, edges, faces := native.Vertices(), native.Edges(), native.Faces()
	if len(supports) != len(faces) || len(circles) != len(edges) {
		return topologyAuthor{}, zero, geom.ErrInvalid
	}
	v := []float64{float64(len(verts))}
	for _, p := range verts {
		v = append(v, p.X, p.Y, p.Z)
	}
	role.vertices = a.addReal(502, 1, v...)
	edgeArgs := []int{504, len(edges)}
	reverse := make([]bool, len(edges))
	for i, e := range edges {
		from, to := e.Start, e.End
		p, q := verts[from], verts[to]
		var id int
		if circles[i].Radius > 0 {
			s := circles[i]
			if directedIGESClockwise(s) {
				from, to, p, q = to, from, q, p
				reverse[i] = true
			}
			id = a.addReal(100, 0, p.Z, s.Center.X, s.Center.Y, p.X, p.Y, q.X, q.Y)
		} else {
			id = a.addReal(110, 0, p.X, p.Y, p.Z, q.X, q.Y, q.Z)
		}
		edgeArgs = append(edgeArgs, id, role.vertices, from+1, role.vertices, to+1)
		role.curveIDs = append(role.curveIDs, id)
	}
	role.edges = a.add(504, 1, topologyNumbers(edgeArgs...))
	x := a.addReal(123, 0, 1, 0, 0)
	z := a.addReal(123, 0, 0, 0, 1)
	shellArgs := []int{514, len(faces)}
	for i, f := range faces {
		if err := ctx.Err(); err != nil {
			return topologyAuthor{}, zero, err
		}
		var support int
		outward := true
		switch {
		case !supports[i].wall:
			h := supports[i].z
			origin := a.addReal(116, 0, 0, 0, h, 0)
			support = a.add(190, 1, topologyNumbers(190, origin, z, x))
			outward = !f.Reversed
		case supports[i].segment.Radius > 0:
			s := supports[i].segment
			origin := a.addReal(116, 0, s.Center.X, s.Center.Y, supports[i].z, 0)
			support = a.addReal(192, 1, float64(origin), float64(z), s.Radius, float64(x))
			outward = !directedIGESClockwise(s)
		default:
			s := supports[i].segment
			dx, ex := cylinderExactSum(s.End.X, -s.Start.X)
			dy, ey := cylinderExactSum(s.End.Y, -s.Start.Y)
			if ex != nil || ey != nil {
				return topologyAuthor{}, zero, geom.ErrUnrepresentable
			}
			origin := a.addReal(116, 0, s.Start.X, s.Start.Y, supports[i].z, 0)
			norm := a.addReal(123, 0, dy, -dx, 0)
			ref := a.addReal(123, 0, dx, dy, 0)
			support = a.add(190, 1, topologyNumbers(190, origin, norm, ref))
		}
		rings := append([][]trimmed.Use{f.Outer}, f.Holes...)
		args := []int{510, support, len(rings), 1}
		for _, ring := range rings {
			uses := []int{508, len(ring)}
			for k := range ring {
				j := k
				// Align the trim traversal with the analytic support. A
				// clockwise wall’s rational native chart points inward.
				reversedChart := supports[i].wall && !outward
				if reversedChart {
					j = len(ring) - 1 - j
				}
				u := ring[j]
				sense := 0
				if (u.Reversed != reverse[u.Edge]) == reversedChart {
					sense = 1
				}
				uses = append(uses, 0, role.edges, u.Edge+1, sense, 0)
			}
			id := a.add(508, 1, topologyNumbers(uses...))
			args = append(args, id)
			if i < 2 {
				role.capLoops[i] = append(role.capLoops[i], id)
			}
		}
		id := a.add(510, 1, topologyNumbers(args...))
		sense := 0
		if outward {
			sense = 1
		}
		shellArgs = append(shellArgs, id, sense)
		role.faceIDs = append(role.faceIDs, id)
	}
	role.shell = a.add(514, 1, topologyNumbers(shellArgs...))
	role.solid = a.add(186, 0, topologyNumbers(186, role.shell, 1, 0))
	for _, p := range verts {
		a.geometry.Entities = append(a.geometry.Entities, Entity{Point: &p})
	}
	if a.err != nil {
		return topologyAuthor{}, zero, a.err
	}
	return a.topologyAuthor, role, nil
}

// WriteExactDirectedCircularLoopExtrusion writes one IGES type-186 solid
// with analytic finite lines and circular arcs and shared topology. The
// reconstructed native shape is proved before the writer receives any bytes.
func WriteExactDirectedCircularLoopExtrusion(ctx context.Context, w io.Writer, d ExactDirectedCircularLoopExtrusion) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	a, _, _, err := buildDirectedIGES(ctx, d)
	if err != nil {
		return fmt.Errorf("iges: exact directed loop: %w", err)
	}
	b, err := a.file(d.Unit)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}
