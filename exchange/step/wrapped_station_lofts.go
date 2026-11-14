package step

import (
	"context"
	"errors"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"sort"
	"strings"
)

// WrappedStationLoftBody retains source walls and exact recovered stations.
// Writer input is Center/Stations, optional exact wall supports, and names; cached Solid/topological IDs are
// not trusted. The source may use whole circle edges; native rims have quarters.
type WrappedStationLoftBody struct {
	Name       string
	Center     geom.Vec2
	Stations   []trimmed.WrappedLoftStation
	Walls      []WrappedSplineFace
	CapNames   [2]string
	Solid      trimmed.Solid
	LengthUnit float64
}

func ReadWrappedStationLoft(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedStationLoftBody, error) {
	return readWrappedStationLoft(r, l, o, true)
}

func readWrappedStationLoft(r io.Reader, l Limits, o WrappedSurfaceOptions, circular bool) (WrappedStationLoftBody, error) {
	fail := func() (WrappedStationLoftBody, error) {
		if !circular {
			return WrappedStationLoftBody{}, errors.New("step: unsupported wrapped rational station loft")
		}
		return WrappedStationLoftBody{}, errors.New("step: unsupported wrapped circular station loft")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	root := int64(0)
	unit := 0.
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") && rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
				return fail()
			}
		}
		rep, e := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if e != nil {
			continue
		}
		ctx, _ := ref(rep.Args[2])
		u, e := contextUnits(f, ctx)
		if e != nil || unit != 0 && unit != u {
			return fail()
		}
		unit = u
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		for _, item := range items {
			if _, e := simple(f, item, "AXIS2_PLACEMENT_3D", 4); e == nil {
				if _, e = placement(f, item); e != nil {
					return fail()
				}
				continue
			}
			if root != 0 {
				return fail()
			}
			if _, e := simple(f, item, "MANIFOLD_SOLID_BREP", 2); e != nil {
				return fail()
			}
			root = item
		}
	}
	if root == 0 {
		return fail()
	}
	body, _ := simple(f, root, "MANIFOLD_SOLID_BREP", 2)
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	shellID, _ := ref(body.Args[1])
	shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	minFaces, maxFaces := 4, 66
	if !circular {
		minFaces, maxFaces = 3, 18
	}
	faces, e := listRefs(shell.Args[1], minFaces)
	if e != nil || len(faces) > maxFaces || len(faces) > l.MaxFaces || 4*(len(faces)-2)+2 > l.MaxEdgeUses || len(faces)-2 > o.MaxSurfaces {
		return fail()
	}
	o.MaxOperations /= len(faces)
	if o.MaxOperations < 1 {
		return WrappedStationLoftBody{}, geom.ErrConvergence
	}
	type band struct {
		face    WrappedSplineFace
		proof   geom.WrappedFrustumEmbedding
		surface int64
	}
	bands := []band{}
	capIDs := []int64{}
	usedFaces := map[int64]bool{}
	usedSurfaces := map[int64]bool{}
	edgeOwners := map[int64][]int64{}
	points := 0
	for _, fid := range faces {
		usedFaces[fid] = true
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, _ := ref(face.Args[2])
		if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); !ok {
			if _, e := simple(f, sid, "PLANE", 2); e != nil {
				return fail()
			}
			capIDs = append(capIDs, fid)
			continue
		}
		if usedSurfaces[sid] {
			return fail()
		}
		usedSurfaces[sid] = true
		view := *f
		view.Entities = map[int64]Entity{}
		for id, ent := range f.Entities {
			view.Entities[id] = ent
		}
		if e = normalizeWrappedIsoSplines(&view, sid, o.MaxPoles); e != nil {
			return WrappedStationLoftBody{}, e
		}
		_, uses, e := analyticFaceLoop(&view, fid, 4)
		if e != nil {
			return fail()
		}
		seen := map[int64]bool{}
		for _, use := range uses {
			if seen[use.id] {
				continue
			}
			seen[use.id] = true
			_, _, gid, e := analyticEdge(f, use.id)
			if e != nil {
				return fail()
			}
			edgeOwners[use.id] = append(edgeOwners[use.id], sid)
			sc, e := simple(f, gid, "SURFACE_CURVE", 4)
			if e != nil {
				continue
			}
			pcs, e := listRefs(sc.Args[2], 2)
			if e != nil || len(pcs) != 2 {
				return fail()
			}
			selected := int64(0)
			for _, pcid := range pcs {
				pc, e := simple(f, pcid, "PCURVE", 3)
				if e != nil {
					return fail()
				}
				owner, _ := ref(pc.Args[1])
				if owner == sid {
					if selected != 0 {
						return fail()
					}
					selected = pcid
				}
			}
			if selected == 0 {
				return fail()
			}
			sc.Args = append([]Value(nil), sc.Args...)
			sc.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: selected}}}
			view.Entities[gid] = Entity{ID: gid, Records: []Record{sc}}
		}
		decoded, e := DecodeWrappedSplineFace(context.Background(), &view, fid, o)
		if e != nil {
			return WrappedStationLoftBody{}, e
		}
		proof, e := geom.CertifyWrappedFrustumEmbedding(context.Background(), decoded.Geometry, geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil || !proof.Complete || circular && !proof.Circular {
			return fail()
		}
		recipe, e := decoded.Geometry.ExportRecipe()
		if e != nil {
			return WrappedStationLoftBody{}, e
		}
		points += len(recipe.Surface.Data().Poles)
		if points > l.MaxVertices {
			return fail()
		}
		bands = append(bands, band{decoded, proof, sid})
	}
	if len(bands) < minFaces-2 || len(capIDs) != 2 {
		return fail()
	}
	sort.Slice(bands, func(i, j int) bool { return bands[i].proof.LowerZ.Cmp(bands[j].proof.LowerZ) < 0 })
	out := WrappedStationLoftBody{Name: name, LengthUnit: unit}
	cx, e := geom.RepresentedExactScalar(bands[0].proof.CenterX)
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	cy, e := geom.RepresentedExactScalar(bands[0].proof.CenterY)
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	out.Center = geom.Vec2{X: cx, Y: cy}
	expectedOwners := map[int64][]int64{}
	vertices := map[int64]bool{}
	seams := map[int64]bool{}
	for i, b := range bands {
		if b.proof.CenterX.Cmp(bands[0].proof.CenterX) != 0 || b.proof.CenterY.Cmp(bands[0].proof.CenterY) != 0 {
			return fail()
		}
		if i > 0 {
			p := bands[i-1]
			if p.proof.UpperZ.Cmp(b.proof.LowerZ) != 0 || p.proof.UpperRadius.Cmp(b.proof.LowerRadius) != 0 || p.face.SideEdges[2] != b.face.SideEdges[0] || p.face.CornerVertices[2] != b.face.CornerVertices[0] {
				return fail()
			}
		}
		seam := b.face.SideEdges[1]
		if seams[seam] {
			return fail()
		}
		seams[seam] = true
		for _, v := range b.face.CornerVertices {
			vertices[v] = true
		}
		for _, side := range []int{0, 2} {
			eid := b.face.SideEdges[side]
			expectedOwners[eid] = append(expectedOwners[eid], b.surface)
		}
		z, e := geom.RepresentedExactScalar(b.proof.LowerZ)
		if e != nil {
			return WrappedStationLoftBody{}, e
		}
		radius, e := geom.RepresentedExactScalar(b.proof.LowerRadius)
		if e != nil {
			return WrappedStationLoftBody{}, e
		}
		out.Stations = append(out.Stations, trimmed.WrappedLoftStation{Z: z, Radius: radius})
		out.Walls = append(out.Walls, b.face)
	}
	last := bands[len(bands)-1]
	z, e := geom.RepresentedExactScalar(last.proof.UpperZ)
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	radius, e := geom.RepresentedExactScalar(last.proof.UpperRadius)
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	out.Stations = append(out.Stations, trimmed.WrappedLoftStation{Z: z, Radius: radius})
	if len(vertices) != len(bands)+1 || len(expectedOwners) != len(bands)+1 || len(edgeOwners) != 2*len(bands)+1 {
		return fail()
	}
	capSeen := [2]bool{}
	for _, fid := range capIDs {
		edge, e := wrappedCylinderCap(f, fid, o)
		if e != nil {
			return WrappedStationLoftBody{}, e
		}
		side := -1
		if edge == bands[0].face.SideEdges[0] {
			side = 0
		}
		if edge == last.face.SideEdges[2] {
			side = 1
		}
		if side < 0 || capSeen[side] {
			return fail()
		}
		capSeen[side] = true
		face, _ := simple(f, fid, "ADVANCED_FACE", 4)
		sense, _ := boolean(face.Args[3])
		if sense != (side == 1) {
			return fail()
		}
		sid, _ := ref(face.Args[2])
		expectedOwners[edge] = append(expectedOwners[edge], sid)
		out.CapNames[side], e = textValue(face.Args[0])
		if e != nil {
			return fail()
		}
	}
	for edge, owners := range expectedOwners {
		if len(owners) != 2 || owners[0] == owners[1] {
			return fail()
		}
		_, _, gid, e := analyticEdge(f, edge)
		if e != nil {
			return fail()
		}
		sc, e := simple(f, gid, "SURFACE_CURVE", 4)
		if e != nil {
			return fail()
		}
		pcs, e := listRefs(sc.Args[2], 2)
		if e != nil || len(pcs) != 2 {
			return fail()
		}
		seen := map[int64]bool{}
		for _, pcid := range pcs {
			pc, e := simple(f, pcid, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			owner, _ := ref(pc.Args[1])
			if seen[owner] || (owner != owners[0] && owner != owners[1]) {
				return fail()
			}
			seen[owner] = true
		}
	}
	for id, ent := range f.Entities {
		for _, rec := range ent.Records {
			if rec.Type == "MANIFOLD_SOLID_BREP" && id != root || rec.Type == "CLOSED_SHELL" && id != shellID || rec.Type == "ADVANCED_FACE" && !usedFaces[id] || rec.Type == "B_SPLINE_SURFACE_WITH_KNOTS" && !usedSurfaces[id] || rec.Type == "BREP_WITH_VOIDS" {
				return fail()
			}
		}
	}
	options := geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	if circular {
		out.Solid, _, e = trimmed.LoftWrappedCircularStations(context.Background(), out.Center, out.Stations, 1e-10, options)
	} else {
		supports := make([]*geom.WrappedSurfacePatch, len(bands))
		for i := range bands {
			supports[i] = bands[i].face.Geometry
		}
		out.Solid, _, e = trimmed.NewWrappedRationalStationLoft(context.Background(), supports, 1e-10, options)
	}
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	return out, nil
}
