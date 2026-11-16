package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

// WriteWrappedCylinderBoreRimFillet writes the authenticated native boundary
// as one analytic STEP solid. Its shared quadrant and seam edges retain the
// original trimming vertices. The receiving kernel can derive each surface's
// angular parameters from the exact model curves and face incidences.
func WriteWrappedCylinderBoreRimFillet(ctx context.Context, w io.Writer, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil || !utf8.ValidString(name) || !validLengthUnit(unit) {
		return geom.ErrInvalid
	}
	g, err := borefillet.Extract(ctx, s, o)
	if err != nil {
		return err
	}
	raw, err := wrappedCylinderBoreRimFilletSTEP(g, name, unit)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if _, err = Parse(bytes.NewReader(raw), Limits{}); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

func wrappedCylinderBoreRimFilletSTEP(g borefillet.Graph, name string, unit float64) ([]byte, error) {
	axis, reference, meridian := borefillet.Vec(0, 0, 1), borefillet.Vec(1, 0, 0), borefillet.Vec(0, -1, 0)
	var numeric error
	number := func(x *big.Rat) string {
		s, err := extrusionDirectionDecimal(x)
		if err != nil {
			numeric = err
			return "0."
		}
		return s
	}
	var raw bytes.Buffer
	err := writeStandalone(&raw, unit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		xyz := func(kind string, v [3]*big.Rat) int64 {
			return emit(fmt.Sprintf("%s('',(%s,%s,%s))", kind, number(v[0]), number(v[1]), number(v[2])))
		}
		point := func(v [3]*big.Rat) int64 { return xyz("CARTESIAN_POINT", v) }
		place := func(origin, normal [3]*big.Rat) int64 {
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", point(origin), xyz("DIRECTION", normal), xyz("DIRECTION", reference)))
		}
		surfaces := make([]int64, len(g.Faces))
		for i, f := range g.Faces {
			p := place(f.Origin, axis)
			switch f.Kind {
			case borefillet.Plane:
				surfaces[i] = emit(fmt.Sprintf("PLANE('',#%d)", p))
			case borefillet.Cylinder:
				surfaces[i] = emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", p, number(f.Major)))
			case borefillet.Torus:
				surfaces[i] = emit(fmt.Sprintf("TOROIDAL_SURFACE('',#%d,%s,%s)", p, number(f.Major), number(f.Minor)))
			}
		}
		vids := make([]int64, len(g.Vertices))
		for i, v := range g.Vertices {
			vids[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point(v)))
		}
		eids := make([]int64, len(g.Edges))
		for i, e := range g.Edges {
			c := e.Circle
			var curve int64
			if c.Radius == nil {
				start, end := g.Vertices[e.Start], g.Vertices[e.End]
				d := axis
				if end[2].Cmp(start[2]) < 0 {
					d[2] = sideRat(-1)
				}
				v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", xyz("DIRECTION", d)))
				curve = emit(fmt.Sprintf("LINE('',#%d,#%d)", point(start), v))
			} else {
				normal := axis
				if c.Meridian {
					normal = meridian
				}
				curve = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", place(c.Center, normal), number(c.Radius)))
			}
			eids[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,%s)", vids[e.Start], vids[e.End], curve, sideDiskBool(e.Forward)))
		}
		fids := make([]int64, len(g.Faces))
		for i, f := range g.Faces {
			bounds := make([]int64, 0, 1+len(f.Holes))
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
			fids[i] = emit(fmt.Sprintf("ADVANCED_FACE('',%s,#%d,%s)", references(bounds), surfaces[i], sideDiskBool(!f.Reversed)))
		}
		shell := emit("CLOSED_SHELL(''," + references(fids) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(name), shell))}
	})
	if err != nil {
		return nil, err
	}
	if numeric != nil {
		return nil, numeric
	}
	return raw.Bytes(), nil
}
