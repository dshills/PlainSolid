package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// ExactCircularLoopExtrusion describes a proved CCW line/minor-circle loop,
// extruded along local Z and placed by a proper signed-coordinate frame.
// Controls may be nonbinary rational; no control is rounded into a STEP spline.
// Segments and Height are the authority, not an unverified native solid cache.
type ExactCircularLoopExtrusion struct {
	Name               string
	Frame              geom.Frame
	Segments           []trimmed.ExactCircularLoopSegment
	Height, LengthUnit float64
}

// WriteExactCircularLoopExtrusion emits a rooted standard analytic B-rep after
// exact native construction and continuous edge certification. Model-only
// LINE/CIRCLE edges carry exact endpoints; no approximate angular pcurves or
// rational-spline controls are invented. Only source scalars within the bounded
// exact-decimal domain and exactly representable posed vertices are accepted.
func WriteExactCircularLoopExtrusion(ctx context.Context, w io.Writer, d ExactCircularLoopExtrusion) error {
	return writeExactCircularLoopDialect(ctx, w, ExactDirectedCircularLoopExtrusion{
		Name: d.Name, Frame: d.Frame, Outer: d.Segments, Height: d.Height, LengthUnit: d.LengthUnit,
	}, false)
}

// ExactDirectedCircularLoopExtrusion stores source circular and linear
// geometry in a signed-coordinate frame. The outer wire is CCW, inner
// wires clockwise; individual minor circular arcs can turn either way.
type ExactDirectedCircularLoopExtrusion struct {
	Name               string
	Frame              geom.Frame
	Outer              []trimmed.ExactCircularLoopSegment
	Holes              [][]trimmed.ExactCircularLoopSegment
	Height, LengthUnit float64
}

// WriteExactDirectedCircularLoopExtrusion preserves each original ring and
// circle sense in a rooted analytic STEP B-rep after exact native validation.
func WriteExactDirectedCircularLoopExtrusion(ctx context.Context, w io.Writer, d ExactDirectedCircularLoopExtrusion) error {
	return writeExactCircularLoopDialect(ctx, w, d, true)
}

func writeExactCircularLoopDialect(ctx context.Context, w io.Writer, d ExactDirectedCircularLoopExtrusion, directed bool) error {
	if ctx == nil || w == nil || !utf8.ValidString(d.Name) || len(d.Name) > 4096 || !planarSplineRigid(frameTransform(d.Frame)) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	var solid trimmed.Solid
	var e error
	if directed {
		solid, e = trimmed.ExtrudeExactDirectedCircularLoops(ctx, d.Outer, d.Holes, min(d.Height, 0), max(d.Height, 0), trimmed.ExactCircularLoopOptions{})
	} else {
		solid, e = trimmed.ExtrudeExactCircularLoop(ctx, d.Outer, d.Height, trimmed.ExactCircularLoopOptions{})
	}
	if e != nil {
		return e
	}
	vertices, edges, faces := solid.Vertices(), solid.Edges(), solid.Faces()
	segments := append([]trimmed.ExactCircularLoopSegment(nil), d.Outer...)
	for _, hole := range d.Holes {
		segments = append(segments, hole...)
	}
	n := len(segments)
	circleForward := func(i int) bool {
		q := segments[i]
		if q.Radius <= 0 {
			return true
		}
		cx, cy := sideRat(q.Center.X), sideRat(q.Center.Y)
		x, y := new(big.Rat).Sub(sideRat(q.Start.X), cx), new(big.Rat).Sub(sideRat(q.Start.Y), cy)
		u, v := new(big.Rat).Sub(sideRat(q.End.X), cx), new(big.Rat).Sub(sideRat(q.End.Y), cy)
		return new(big.Rat).Sub(new(big.Rat).Mul(x, v), new(big.Rat).Mul(y, u)).Sign() > 0
	}
	if len(vertices) != 2*n || len(edges) != 3*n || len(faces) != n+2 {
		return geom.ErrInvalid
	}
	world := make([]geom.Vec3, len(vertices))
	for i, p := range vertices {
		world[i], e = sideDiskWorld(d.Frame, p)
		if e != nil {
			return e
		}
	}
	var numeric error
	number := func(q *big.Rat) string {
		if q == nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
			numeric = geom.ErrUnrepresentable
			return "0."
		}
		s, e := extrusionDirectionDecimal(q)
		if e != nil {
			numeric = e
			return "0."
		}
		if !strings.Contains(s, ".") {
			s += "."
		}
		return s
	}
	vec := func(p geom.Vec3) [3]*big.Rat { return [3]*big.Rat{sideRat(p.X), sideRat(p.Y), sideRat(p.Z)} }
	rotate := func(v [3]*big.Rat) [3]*big.Rat {
		a := frameTransform(d.Frame)
		var out [3]*big.Rat
		for i := range out {
			out[i] = new(big.Rat)
			for j := 0; j < 3; j++ {
				out[i].Add(out[i], new(big.Rat).Mul(sideRat(a[i][j]), v[j]))
			}
		}
		return out
	}
	var staged bytes.Buffer
	e = writeStandalone(&staged, d.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		xyz := func(kind string, v [3]*big.Rat) int64 {
			return emit(fmt.Sprintf("%s('',(%s,%s,%s))", kind, number(v[0]), number(v[1]), number(v[2])))
		}
		point := func(p geom.Vec3) int64 { return xyz("CARTESIAN_POINT", vec(p)) }
		place := func(origin geom.Vec3, z, x [3]*big.Rat) int64 {
			p, e := sideDiskWorld(d.Frame, origin)
			if e != nil {
				numeric = e
			}
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", point(p), xyz("DIRECTION", rotate(z)), xyz("DIRECTION", rotate(x))))
		}
		axis := vec(geom.Vec3{Z: 1})
		ref := vec(geom.Vec3{X: 1})
		surfaces := make([]int64, n+2)
		for j := 0; j < 2; j++ {
			surfaces[j] = emit(fmt.Sprintf("PLANE('',#%d)", place(geom.Vec3{Z: vertices[j*n].Z}, axis, ref)))
		}
		for i, s := range segments {
			if s.Radius > 0 {
				surfaces[2+i] = emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", place(geom.Vec3{X: s.Center.X, Y: s.Center.Y}, axis, ref), number(sideRat(s.Radius))))
			} else {
				dx := new(big.Rat).Sub(sideRat(s.End.X), sideRat(s.Start.X))
				dy := new(big.Rat).Sub(sideRat(s.End.Y), sideRat(s.Start.Y))
				t := [3]*big.Rat{dx, dy, new(big.Rat)}
				normal := [3]*big.Rat{new(big.Rat).Set(dy), new(big.Rat).Neg(dx), new(big.Rat)}
				surfaces[2+i] = emit(fmt.Sprintf("PLANE('',#%d)", place(geom.Vec3{X: s.Start.X, Y: s.Start.Y}, normal, t)))
			}
		}
		vids := make([]int64, len(vertices))
		for i, p := range world {
			vids[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point(p)))
		}
		eids := make([]int64, len(edges))
		for i, edge := range edges {
			var model int64
			if i < 2*n && segments[i%n].Radius > 0 {
				s := segments[i%n]
				model = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", place(geom.Vec3{X: s.Center.X, Y: s.Center.Y, Z: vertices[edge.Start].Z}, axis, ref), number(sideRat(s.Radius))))
			} else {
				a, b := vec(world[edge.Start]), vec(world[edge.End])
				var direction [3]*big.Rat
				for j := range direction {
					direction[j] = new(big.Rat).Sub(b[j], a[j])
				}
				v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", xyz("DIRECTION", direction)))
				model = emit(fmt.Sprintf("LINE('',#%d,#%d)", point(world[edge.Start]), v))
			}
			sameSense := i >= 2*n || circleForward(i%n)
			eids[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,%s)", vids[edge.Start], vids[edge.End], model, sideDiskBool(sameSense)))
		}
		fids := make([]int64, len(faces))
		for i, f := range faces {
			var bounds []int64
			for j, wire := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
				uses := make([]int64, len(wire))
				for k, u := range wire {
					uses[k] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", eids[u.Edge], sideDiskBool(!u.Reversed)))
				}
				loop := emit("EDGE_LOOP(''," + references(uses) + ")")
				kind := "FACE_OUTER_BOUND"
				if j > 0 {
					kind = "FACE_BOUND"
				}
				bounds = append(bounds, emit(fmt.Sprintf("%s('',#%d,%s)", kind, loop, sideDiskBool(!f.Reversed))))
			}
			sameSense := !f.Reversed
			if i >= 2 && !circleForward(i-2) {
				sameSense = !sameSense
			}
			fids[i] = emit(fmt.Sprintf("ADVANCED_FACE('',%s,#%d,%s)", references(bounds), surfaces[i], sideDiskBool(sameSense)))
		}
		shell := emit("CLOSED_SHELL(''," + references(fids) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(d.Name), shell))}
	})
	if e != nil {
		return e
	}
	if numeric != nil {
		return numeric
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	marker := []byte("PRODUCT('1','geometry','',")
	if bytes.Count(staged.Bytes(), marker) != 1 {
		return geom.ErrInvalid
	}
	kind := "exact-circular-loop-extrusion"
	if directed {
		kind = "exact-directed-circular-loop-extrusion"
	}
	raw := bytes.Replace(staged.Bytes(), marker, []byte("PRODUCT('"+kind+"',"+quote(d.Name)+",'',"), 1)
	if _, e = Parse(bytes.NewReader(raw), Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxValues: 65536, MaxDepth: 16}); e != nil {
		return e
	}
	k, e := w.Write(raw)
	if e == nil && k != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
