package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/annularchamfer"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

// WriteEccentricAnnularChamfer writes one exact four-rim-chamfered circular
// annular frustum. Its wall supports revolve original straight generators, so
// even nonrepresentable cone angles preserve the exact authored radial slope.
// The model edges keep the original circular quarters and generators.
func WriteEccentricAnnularChamfer(ctx context.Context, w io.Writer, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil || !utf8.ValidString(name) || !validLengthUnit(unit) {
		return geom.ErrInvalid
	}
	g, err := annularchamfer.Extract(ctx, s, o)
	if err != nil {
		return err
	}
	var raw bytes.Buffer
	var decimalErr error
	number := func(q *big.Rat) string {
		s, err := extrusionDirectionDecimal(q)
		if err != nil {
			decimalErr = err
			return "0."
		}
		return s
	}
	err = writeStandaloneWithUnit(&raw, unit, 0, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p [3]*big.Rat) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", number(p[0]), number(p[1]), number(p[2])))
		}
		dir := func(d [3]*big.Rat) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", number(d[0]), number(d[1]), number(d[2])))
		}
		axis, ref := dir(borefillet.Vec(0, 0, 1)), dir(borefillet.Vec(1, 0, 0))
		frame := func(p [3]*big.Rat) int64 {
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", point(p), axis, ref))
		}
		line := func(a, b [3]*big.Rat) int64 {
			var d [3]*big.Rat
			for i := range d {
				d[i] = new(big.Rat).Sub(b[i], a[i])
			}
			v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", dir(d)))
			return emit(fmt.Sprintf("LINE('',#%d,#%d)", point(a), v))
		}
		surfaces := make([]int64, len(g.Faces))
		for i, f := range g.Faces {
			b := g.Bands[i]
			if b.Patch == nil {
				surfaces[i] = emit(fmt.Sprintf("PLANE('',#%d)", frame(f.Origin)))
				continue
			}
			center := [3]*big.Rat{b.X, b.Y, b.Z[0]}
			low := [3]*big.Rat{new(big.Rat).Add(b.X, b.R[0]), b.Y, b.Z[0]}
			high := [3]*big.Rat{new(big.Rat).Add(b.X, b.R[1]), b.Y, b.Z[1]}
			generator := line(low, high)
			spin := emit(fmt.Sprintf("AXIS1_PLACEMENT('',#%d,#%d)", point(center), axis))
			surfaces[i] = emit(fmt.Sprintf("SURFACE_OF_REVOLUTION('',#%d,#%d)", generator, spin))
		}
		vid := make([]int64, len(g.Vertices))
		for i, p := range g.Vertices {
			vid[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point(p)))
		}
		eid := make([]int64, len(g.Edges))
		for i, e := range g.Edges {
			var curve int64
			if e.Circle.Radius == nil {
				curve = line(g.Vertices[e.Start], g.Vertices[e.End])
			} else {
				curve = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", frame(e.Circle.Center), number(e.Circle.Radius)))
			}
			eid[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,%s)", vid[e.Start], vid[e.End], curve, sideDiskBool(e.Forward)))
		}
		fid := make([]int64, len(g.Faces))
		for i, f := range g.Faces {
			bounds := make([]int64, 0, 1+len(f.Holes))
			for j, wire := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
				uses := make([]int64, len(wire))
				for k, u := range wire {
					uses[k] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", eid[u.Edge], sideDiskBool(!u.Reversed)))
				}
				loop := emit("EDGE_LOOP(''," + references(uses) + ")")
				kind := "FACE_OUTER_BOUND"
				if j > 0 {
					kind = "FACE_BOUND"
				}
				bounds = append(bounds, emit(fmt.Sprintf("%s('',#%d,%s)", kind, loop, sideDiskBool(!f.Reversed))))
			}
			fid[i] = emit(fmt.Sprintf("ADVANCED_FACE('',%s,#%d,%s)", references(bounds), surfaces[i], sideDiskBool(!f.Reversed)))
		}
		shell := emit("CLOSED_SHELL(''," + references(fid) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(name), shell))}
	}, emitExactPlanarLengthUnit)
	if err != nil {
		return err
	}
	if decimalErr != nil {
		return decimalErr
	}
	if _, err := Parse(bytes.NewReader(raw.Bytes()), Limits{}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw.Bytes())
	if err == nil && n != raw.Len() {
		return io.ErrShortWrite
	}
	return err
}
