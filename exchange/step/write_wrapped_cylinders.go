package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// WriteWrappedCylindricalBodies regenerates full closed source rim edges and
// exact planar rational cap pcurves. Material proof is mandatory before output.
func WriteWrappedCylindricalBodies(w io.Writer, m WrappedCylindricalModel) error {
	return writeWrappedCappedBodies(w, m, false)
}

func writeWrappedCappedBodies(w io.Writer, m WrappedCylindricalModel, frustum bool) error {
	return stageWrappedCappedBodies(w, m, func(raw []byte) error {
		_, e := readWrappedCappedBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}, frustum)
		return e
	})
}

// Private graph staging only. Public material writers must independently prove
// their complete output; a staged cap does not certify a material solid.
func stageWrappedCappedBodies(w io.Writer, m WrappedCylindricalModel, validate func([]byte) error) error {

	if w == nil || len(m.Bodies) == 0 || len(m.Bodies) > 64 {
		return fmt.Errorf("step: invalid wrapped cylinder model")
	}
	surface := WrappedSurfaceModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		surface.Surfaces = append(surface.Surfaces, WrappedSplineSurface{Name: b.Face.Name, Geometry: b.Face.Geometry})
	}
	var raw bytes.Buffer
	if e := WriteWrappedSplineFaces(&raw, surface); e != nil {
		return e
	}
	source := raw.String()
	f, e := Parse(strings.NewReader(source), Limits{})
	if e != nil {
		return e
	}
	next := int64(1)
	ids := map[int64]int64{}
	for _, id := range f.Order {
		ids[id] = id
		if id >= next {
			next = id + 1
		}
	}
	extra := []string{}
	emit := func(record string) int64 {
		id := next
		next++
		ids[id] = id
		extra = append(extra, fmt.Sprintf("#%d=%s;", id, record))
		return id
	}
	groups := [][3]int64{}
	for _, fid := range f.Order {
		ent := f.Entities[fid]
		if len(ent.Records) != 1 || ent.Records[0].Type != "ADVANCED_FACE" {
			continue
		}
		face, e := DecodeWrappedSplineFace(context.Background(), f, fid, WrappedSurfaceOptions{})
		if e != nil {
			return e
		}
		group := [3]int64{fid}
		for k, side := range []int{0, 2} {
			eid := face.SideEdges[side]
			_, _, gid, e := analyticEdge(f, eid)
			if e != nil {
				return e
			}
			sc, e := simple(f, gid, "SURFACE_CURVE", 4)
			if e != nil {
				return e
			}
			cid, _ := ref(sc.Args[1])
			curve, e := DecodeBSplineCurve(f, cid, 4096)
			if e != nil {
				return e
			}
			poles := curve.Geometry.Poles()
			z := poles[0].Z
			for _, p := range poles {
				if p.Z != z {
					return fmt.Errorf("step: nonplanar cap rim")
				}
			}
			origin := emit(fmt.Sprintf("CARTESIAN_POINT('',(0.,0.,%s))", real(z)))
			axis := emit("DIRECTION('',(0.,0.,1.))")
			refdir := emit("DIRECTION('',(1.,0.,0.))")
			frame := emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", origin, axis, refdir))
			plane := emit(fmt.Sprintf("PLANE('',#%d)", frame))
			base, ok := component(f.Entities[cid], "B_SPLINE_CURVE")
			index := 1
			if !ok {
				base, _ = component(f.Entities[cid], "B_SPLINE_CURVE_WITH_KNOTS")
				index = 2
			}
			pointIDs, e := listRefs(base.Args[index], 2)
			if e != nil {
				return e
			}
			remap := map[int64]int64{}
			for _, pid := range pointIDs {
				if remap[pid] != 0 {
					continue
				}
				p, e := triple(f, pid, "CARTESIAN_POINT")
				if e != nil {
					return e
				}
				remap[pid] = emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(p.X), real(p.Y)))
			}
			pcCurve := emit(assemblyEntity(f.Entities[cid], remap))
			ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
			rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", pcCurve, ctx))
			pc := emit(fmt.Sprintf("PCURVE('',#%d,#%d)", plane, rep))
			sc.Args = append([]Value(nil), sc.Args...)
			sc.Args[2] = Value{Kind: Aggregate, Items: append(append([]Value(nil), sc.Args[2].Items...), Value{Kind: Reference, Ref: pc})}
			f.Entities[gid] = Entity{ID: gid, Records: []Record{sc}}
			use := emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,.T.)", eid))
			loop := emit(fmt.Sprintf("EDGE_LOOP('',(#%d))", use))
			sense := ".F."
			if k == 1 {
				sense = ".T."
			}
			bound := emit(fmt.Sprintf("FACE_OUTER_BOUND('',#%d,%s)", loop, sense))
			group[k+1] = emit(fmt.Sprintf("ADVANCED_FACE(%s,(#%d),#%d,%s)", quote(m.Bodies[len(groups)].CapNames[k]), bound, plane, sense))
		}
		groups = append(groups, group)
	}
	var stage strings.Builder
	stage.WriteString(source[:strings.Index(source, "DATA;")+5])
	stage.WriteByte('\n')
	for _, id := range f.Order {
		fmt.Fprintf(&stage, "#%d=%s;\n", id, assemblyEntity(f.Entities[id], ids))
	}
	for _, line := range extra {
		stage.WriteString(line)
		stage.WriteByte('\n')
	}
	stage.WriteString("ENDSEC;\nEND-ISO-10303-21;\n")
	f, e = Parse(strings.NewReader(stage.String()), Limits{})
	if e != nil {
		return e
	}
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		roots := []int64{}
		for i, group := range groups {
			fs := copySplineGeometry(emit, f, group[:])
			shell := emit("CLOSED_SHELL(''," + references(fs) + ")")
			roots = append(roots, emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(m.Bodies[i].Name), shell)))
		}
		return roots
	})
	if e != nil {
		return e
	}
	if validate != nil {
		if e = validate(out.Bytes()); e != nil {
			return e
		}
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}
