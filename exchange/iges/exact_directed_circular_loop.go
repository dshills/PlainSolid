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
	a := directedIGESAuthor{}
	role := directedIGESRoles{}
	verts, edges, faces := native.Vertices(), native.Edges(), native.Faces()
	flat := append([]trimmed.ExactCircularLoopSegment(nil), d.Outer...)
	role.ringOffsets = []int{0, len(flat)}
	for _, ring := range d.Holes {
		flat = append(flat, ring...)
		role.ringOffsets = append(role.ringOffsets, len(flat))
	}
	n := len(flat)
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
		if i < 2*n && flat[i%n].Radius > 0 {
			s := flat[i%n]
			if directedIGESClockwise(s) {
				from, to, p, q = to, from, q, p
				reverse[i] = true
			}
			id = a.addReal(100, 0, p.Z, s.Center.X, s.Center.Y, p.X, p.Y, q.X, q.Y)
		} else {
			id = a.addReal(110, 0, p.X, p.Y, p.Z, q.X, q.Y, q.Z)
		}
		edgeArgs = append(edgeArgs, id, role.vertices, from+1, role.vertices, to+1)
		switch {
		case i < n:
			role.rims[0] = append(role.rims[0], id)
		case i < 2*n:
			role.rims[1] = append(role.rims[1], id)
		default:
			role.verticals = append(role.verticals, id)
		}
	}
	role.edges = a.add(504, 1, topologyNumbers(edgeArgs...))
	x := a.addReal(123, 0, 1, 0, 0)
	z := a.addReal(123, 0, 0, 0, 1)
	shellArgs := []int{514, len(faces)}
	for i, f := range faces {
		if err = ctx.Err(); err != nil {
			return topologyAuthor{}, zero, trimmed.Solid{}, err
		}
		var support int
		outward := true
		switch {
		case i < 2:
			h := []float64{d.Bottom, d.Top}[i]
			origin := a.addReal(116, 0, 0, 0, h, 0)
			support = a.add(190, 1, topologyNumbers(190, origin, z, x))
			outward = i == 1
		case flat[i-2].Radius > 0:
			s := flat[i-2]
			origin := a.addReal(116, 0, s.Center.X, s.Center.Y, d.Bottom, 0)
			support = a.addReal(192, 1, float64(origin), float64(z), s.Radius, float64(x))
			outward = !directedIGESClockwise(s)
		default:
			s := flat[i-2]
			dx, ex := cylinderExactSum(s.End.X, -s.Start.X)
			dy, ey := cylinderExactSum(s.End.Y, -s.Start.Y)
			if ex != nil || ey != nil {
				return topologyAuthor{}, zero, trimmed.Solid{}, geom.ErrUnrepresentable
			}
			origin := a.addReal(116, 0, s.Start.X, s.Start.Y, d.Bottom, 0)
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
				reversedChart := i >= 2 && !outward
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
		if i < 2 {
			role.caps[i] = id
		} else {
			role.walls = append(role.walls, id)
		}
	}
	role.shell = a.add(514, 1, topologyNumbers(shellArgs...))
	role.solid = a.add(186, 0, topologyNumbers(186, role.shell, 1, 0))
	for _, p := range verts {
		a.geometry.Entities = append(a.geometry.Entities, Entity{Point: &p})
	}
	if a.err != nil {
		return topologyAuthor{}, zero, trimmed.Solid{}, a.err
	}
	return a.topologyAuthor, role, native, nil
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
