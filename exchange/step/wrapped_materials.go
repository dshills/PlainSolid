package step

import (
	"bytes"
	"context"
	"errors"
	"io"
	"plainsolid/geom"
	"strings"
)

// WrappedMaterialBody is a closed tagged union: exactly one payload must be
// present. SourceBodyID is read-only provenance and is regenerated on write.
// Payloads describe independent certified bodies, never assembly instances.
type WrappedMaterialBody struct {
	SourceBodyID   int64
	Toroid         *WrappedToroidalBody
	Cylinder       *WrappedCylindricalBody
	Frustum        *WrappedFrustumBody
	Tube           *WrappedTubeBody
	BlindBore      *WrappedBlindBoreBody
	Shoulder       *WrappedShoulderBody
	StationLoft    *WrappedStationLoftBody
	StationBore    *WrappedStationBoreBody
	AnnularFrustum *WrappedAnnularFrustumBody
	RimFillet      *WrappedRimFilletBody
	Cavity         *WrappedCavityBody
	ToroidalVoid   *WrappedToroidalVoid
}
type WrappedMaterialModel struct {
	SourceSchemas []string // Read-only source FILE_SCHEMA metadata; not a conformance claim.
	Bodies        []WrappedMaterialBody
	LengthUnit    float64
}

// ReadWrappedMaterials dispatches by explicit graph and exact embedding
// certificate. It does not flatten assemblies or probe unrelated shape readers.
func ReadWrappedMaterials(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedMaterialModel, error) {
	fail := func() (WrappedMaterialModel, error) {
		return WrappedMaterialModel{}, errors.New("step: unsupported mixed wrapped material graph")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedMaterialModel{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return WrappedMaterialModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedMaterialModel{}, e
	}
	pointCount := 0
	for _, ent := range f.Entities {
		for _, rec := range ent.Records {
			if rec.Type == "CARTESIAN_POINT" {
				pointCount++
			}
		}
	}
	if pointCount > l.MaxVertices {
		return fail()
	}
	out := WrappedMaterialModel{}
	for _, rec := range f.Header {
		if rec.Type == "FILE_SCHEMA" {
			if len(rec.Args) != 1 || rec.Args[0].Kind != Aggregate {
				return fail()
			}
			for _, v := range rec.Args[0].Items {
				schema, e := textValue(v)
				if e != nil {
					return fail()
				}
				out.SourceSchemas = append(out.SourceSchemas, schema)
			}
		}
	}

	roots := []int64{}
	contexts := []int64{}
	repIDs := []int64{}
	used := map[int64]bool{}
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
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return fail()
		}
		unit, e := contextUnits(f, ctx)
		if e != nil || out.LengthUnit != 0 && out.LengthUnit != unit {
			return fail()
		}
		out.LengthUnit = unit
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
			_, ordinary := simple(f, item, "MANIFOLD_SOLID_BREP", 2)
			_, void := simple(f, item, "BREP_WITH_VOIDS", 3)
			if (ordinary != nil && void != nil) || used[item] {
				return fail()
			}
			used[item] = true
			roots = append(roots, item)
			contexts = append(contexts, ctx)
			repIDs = append(repIDs, id)
		}
	}
	if len(roots) == 0 || len(roots) > l.MaxSolids || len(roots) > o.MaxSurfaces {
		return fail()
	}
	o.MaxOperations /= len(roots)
	if o.MaxOperations < 1 {
		return WrappedMaterialModel{}, geom.ErrConvergence
	}
	ownedTopology := map[int64]bool{}
	edgeUseCount := 0
	ownedFaces := map[int64]bool{}
	ownedSurfaces := map[int64]bool{}
	ownedShells := map[int64]bool{}
	for i, root := range roots {
		body, e := simple(f, root, "MANIFOLD_SOLID_BREP", 2)
		isVoid := e != nil
		voidFaceCount := 0
		var faces []int64
		shellIDs := []int64{}
		if isVoid {
			body, e = simple(f, root, "BREP_WITH_VOIDS", 3)
			if e != nil {
				return fail()
			}
			vs, e := listRefs(body.Args[2], 1)
			if e != nil || len(vs) != 1 || ownedTopology[vs[0]] {
				return fail()
			}
			oriented, e := simple(f, vs[0], "ORIENTED_CLOSED_SHELL", 4)
			if e != nil || oriented.Args[1].Kind != Derived {
				return fail()
			}
			sense, e := boolean(oriented.Args[3])
			if e != nil || sense {
				return fail()
			}
			inner, e := ref(oriented.Args[2])
			if e != nil {
				return fail()
			}
			shellIDs = append(shellIDs, inner)
		}
		outer, e := ref(body.Args[1])
		if e != nil {
			return fail()
		}
		shellIDs = append([]int64{outer}, shellIDs...)
		for _, shellID := range shellIDs {
			shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
			if e != nil || ownedShells[shellID] {
				return fail()
			}
			ownedShells[shellID] = true
			fs, e := listRefs(shell.Args[1], 1)
			if e != nil {
				return fail()
			}
			if isVoid {
				if len(fs) != 1 && len(fs) != 3 {
					return fail()
				}
				if voidFaceCount != 0 && len(fs) != voidFaceCount {
					return fail()
				}
				voidFaceCount = len(fs)
			} else if len(fs) != 1 && (len(fs) < 3 || len(fs) > 130) {
				return fail()
			}
			faces = append(faces, fs...)
		}
		sid := int64(0)
		wallCount, positiveWalls, annuli, quadraticWalls := 0, 0, 0, 0
		for _, fid := range faces {
			if ownedFaces[fid] {
				return fail()
			}
			ownedFaces[fid] = true
			face, e := simple(f, fid, "ADVANCED_FACE", 4)
			if e != nil {
				return fail()
			}
			faceBounds, e := listRefs(face.Args[1], 1)
			if e != nil {
				return fail()
			}
			for _, bid := range faceBounds {
				ent := f.Entities[bid]
				if len(ent.Records) != 1 || (ent.Records[0].Type != "FACE_BOUND" && ent.Records[0].Type != "FACE_OUTER_BOUND") {
					return fail()
				}
				bound, e := simple(f, bid, ent.Records[0].Type, 3)
				if e != nil {
					return fail()
				}
				lid, e := ref(bound.Args[1])
				if e != nil {
					return fail()
				}
				loop, e := simple(f, lid, "EDGE_LOOP", 2)
				if e != nil {
					return fail()
				}
				uses, e := listRefs(loop.Args[1], 1)
				if e != nil || len(uses) > l.MaxEdgeUses-edgeUseCount {
					return fail()
				}
				edgeUseCount += len(uses)
			}
			support, _ := ref(face.Args[2])
			if _, ok := component(f.Entities[support], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
				if ownedSurfaces[support] {
					return fail()
				}
				wallCount++
				base, complex := component(f.Entities[support], "B_SPLINE_SURFACE")
				degreeIndex := 1
				if !complex {
					base, _ = component(f.Entities[support], "B_SPLINE_SURFACE_WITH_KNOTS")
					degreeIndex = 2
				}
				if len(base.Args) <= degreeIndex {
					return fail()
				}
				degree, e := integer(base.Args[degreeIndex])
				if e != nil {
					return fail()
				}
				if degree == 2 {
					quadraticWalls++
				}

				sense, e := boolean(face.Args[3])
				if e != nil {
					return fail()
				}
				if sense {
					positiveWalls++
				}
				sid = support
				ownedSurfaces[support] = true
			} else {
				if _, e := simple(f, support, "PLANE", 2); e != nil {
					return fail()
				}
				bounds, e := listRefs(face.Args[1], 1)
				if e != nil || len(bounds) > 2 {
					return fail()
				}
				if len(bounds) == 2 {
					annuli++
				}
			}
		}
		isStationBore := !isVoid && len(faces) >= 5 && wallCount == len(faces)-2 && positiveWalls >= 1 && wallCount-positiveWalls >= 1 && annuli == 2
		isLoft := !isVoid && len(faces) >= 4 && wallCount == len(faces)-2 && positiveWalls == wallCount && annuli == 0
		if sid == 0 || (!isVoid && len(faces) <= 3 && wallCount != 1) || ((isVoid || len(faces) >= 4) && wallCount != 2 && !isLoft && !isStationBore) {
			return fail()
		}
		// Retain source IDs while isolating only the selected body's dependency
		// closure and its explicit representation context. Cycles fail closed.
		local := &File{Entities: map[int64]Entity{}}
		visiting := map[int64]bool{}
		var copyOne func(int64) error
		var walk func(Value) error
		walk = func(v Value) error {
			if v.Kind == Reference {
				return copyOne(v.Ref)
			}
			for _, x := range v.Items {
				if e := walk(x); e != nil {
					return e
				}
			}
			return nil
		}
		copyOne = func(id int64) error {
			if visiting[id] || len(visiting) >= l.MaxDepth {
				return errors.New("step: cyclic material dependency")
			}
			if _, ok := local.Entities[id]; ok {
				return nil
			}
			ent, ok := f.Entities[id]
			if !ok {
				return errors.New("step: missing material dependency")
			}
			visiting[id] = true
			for _, r := range ent.Records {
				for _, v := range r.Args {
					if e := walk(v); e != nil {
						return e
					}
				}
			}
			delete(visiting, id)
			local.Entities[id] = ent
			return nil
		}
		if e = copyOne(root); e != nil {
			return WrappedMaterialModel{}, e
		}
		for id, ent := range local.Entities {
			if len(ent.Records) != 1 {
				continue
			}
			switch ent.Records[0].Type {
			case "ORIENTED_CLOSED_SHELL", "EDGE_CURVE", "VERTEX_POINT", "ORIENTED_EDGE", "EDGE_LOOP", "FACE_BOUND", "FACE_OUTER_BOUND":
				if ownedTopology[id] {
					return fail()
				}
				ownedTopology[id] = true
			}
		}
		if e = copyOne(contexts[i]); e != nil {
			return WrappedMaterialModel{}, e
		}
		rep, _ := simple(f, repIDs[i], "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		rep.Args = append([]Value(nil), rep.Args...)
		rep.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: root}}}
		local.Entities[repIDs[i]] = Entity{ID: repIDs[i], Records: []Record{rep}}
		for _, id := range f.Order {
			if _, ok := local.Entities[id]; ok {
				local.Order = append(local.Order, id)
			}
		}
		raw := serializeWrappedProofFile(local)
		result := WrappedMaterialBody{SourceBodyID: root}
		if isVoid {
			if positiveWalls != 2 || annuli != 0 {
				return fail()
			}
			if voidFaceCount == 1 {
				m, e := ReadWrappedToroidalVoid(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.ToroidalVoid = &m
			} else {
				m, e := ReadWrappedCavity(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.Cavity = &m
			}
		} else if len(faces) == 1 {
			m, e := ReadWrappedToroidalBodies(bytes.NewReader(raw), l, o)
			if e != nil {
				return WrappedMaterialModel{}, e
			}
			if len(m.Bodies) != 1 {
				return fail()
			}
			result.Toroid = &m.Bodies[0]
		} else if len(faces) >= 4 {
			switch {
			case isStationBore:
				body, e := ReadWrappedStationBore(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.StationBore = &body
			case isLoft && quadraticWalls > 0:
				if wallCount > 3 || quadraticWalls != wallCount-1 {
					return fail()
				}
				body, e := ReadWrappedRimFillet(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.RimFillet = &body
			case isLoft:
				m, e := ReadWrappedStationLoft(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.StationLoft = &m
			case len(faces) == 4 && positiveWalls == 1 && annuli == 2:
				tapered := false
				for _, fid := range faces {
					face, _ := simple(f, fid, "ADVANCED_FACE", 4)
					support, _ := ref(face.Args[2])
					if _, ok := component(f.Entities[support], "B_SPLINE_SURFACE_WITH_KNOTS"); !ok {
						continue
					}
					decoded, err := DecodeWrappedSurface(context.Background(), f, support, o)
					if err != nil {
						return WrappedMaterialModel{}, err
					}
					proof, err := geom.CertifyWrappedFrustumEmbedding(context.Background(), decoded.Geometry, geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
					if err != nil {
						return WrappedMaterialModel{}, err
					}
					if !proof.Complete {
						return WrappedMaterialModel{}, geom.ErrConvergence
					}
					tapered = tapered || proof.LowerRadius.Cmp(proof.UpperRadius) != 0
				}
				if tapered {
					body, err := ReadWrappedAnnularFrustum(bytes.NewReader(raw), l, o)
					if err != nil {
						return WrappedMaterialModel{}, err
					}
					result.AnnularFrustum = &body
				} else {
					body, err := ReadWrappedTube(bytes.NewReader(raw), l, o)
					if err != nil {
						return WrappedMaterialModel{}, err
					}
					result.Tube = &body
				}
			case len(faces) == 5 && positiveWalls == 1 && annuli == 1:
				m, e := ReadWrappedBlindBore(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.BlindBore = &m
			case len(faces) == 5 && positiveWalls == 2 && annuli == 1:
				m, e := ReadWrappedShoulder(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.Shoulder = &m
			default:
				return fail()
			}
		} else {
			support, e := DecodeWrappedSurface(context.Background(), f, sid, o)
			if e != nil {
				return WrappedMaterialModel{}, e
			}
			proof, e := geom.CertifyWrappedFrustumEmbedding(context.Background(), support.Geometry, geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil || !proof.Complete {
				return fail()
			}
			if proof.LowerRadius.Cmp(proof.UpperRadius) == 0 {
				m, e := ReadWrappedCylindricalBodies(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.Cylinder = &m.Bodies[0]
			} else {
				m, e := ReadWrappedFrusta(bytes.NewReader(raw), l, o)
				if e != nil {
					return WrappedMaterialModel{}, e
				}
				result.Frustum = &m.Bodies[0]
			}
		}
		out.Bodies = append(out.Bodies, result)
	}
	if len(ownedFaces) > l.MaxFaces || edgeUseCount > l.MaxEdgeUses || len(ownedSurfaces) > o.MaxSurfaces {
		return fail()
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if r.Type == "MANIFOLD_SOLID_BREP" && !used[id] || r.Type == "ADVANCED_FACE" && !ownedFaces[id] || r.Type == "CLOSED_SHELL" && !ownedShells[id] || r.Type == "B_SPLINE_SURFACE_WITH_KNOTS" && !ownedSurfaces[id] || r.Type == "BREP_WITH_VOIDS" && !used[id] || r.Type == "ORIENTED_CLOSED_SHELL" && !ownedTopology[id] {
				return fail()
			}
		}
	}
	return out, nil
}

// WriteWrappedMaterials writes independent named bodies without introducing
// assembly semantics. Payload certificates and source graphs are revalidated.
func WriteWrappedMaterials(w io.Writer, m WrappedMaterialModel) error {
	if w == nil || len(m.Bodies) == 0 || len(m.Bodies) > 64 {
		return geom.ErrInvalid
	}
	files := []*File{}
	roots := []int64{}
	for _, b := range m.Bodies {
		count := 0
		var raw bytes.Buffer
		var e error
		if b.Toroid != nil {
			count++
			e = WriteWrappedToroidalBodies(&raw, WrappedToroidalModel{Bodies: []WrappedToroidalBody{*b.Toroid}, LengthUnit: m.LengthUnit})
		}
		if b.Cylinder != nil {
			count++
			if count == 1 {
				e = WriteWrappedCylindricalBodies(&raw, WrappedCylindricalModel{Bodies: []WrappedCylindricalBody{*b.Cylinder}, LengthUnit: m.LengthUnit})
			}
		}
		if b.Frustum != nil {
			count++
			if count == 1 {
				e = WriteWrappedFrusta(&raw, WrappedFrustumModel{Bodies: []WrappedFrustumBody{*b.Frustum}, LengthUnit: m.LengthUnit})
			}
		}
		if b.Tube != nil {
			count++
			if count == 1 {
				q := *b.Tube
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedTube(&raw, q)
			}
		}
		if b.BlindBore != nil {
			count++
			if count == 1 {
				q := *b.BlindBore
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedBlindBore(&raw, q)
			}
		}
		if b.Shoulder != nil {
			count++
			if count == 1 {
				q := *b.Shoulder
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedShoulder(&raw, q)
			}
		}
		if b.Cavity != nil {
			count++
			if count == 1 {
				q := *b.Cavity
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedCavity(&raw, q)
			}
		}
		if b.ToroidalVoid != nil {
			count++
			if count == 1 {
				q := *b.ToroidalVoid
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedToroidalVoid(&raw, q)
			}
		}
		if b.StationLoft != nil {
			count++
			if count == 1 {
				q := *b.StationLoft
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedStationLoft(&raw, q)
			}
		}
		if b.AnnularFrustum != nil {
			count++
			if count == 1 {
				q := *b.AnnularFrustum
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedAnnularFrustum(&raw, q)
			}
		}
		if b.StationBore != nil {
			count++
			if count == 1 {
				q := *b.StationBore
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedStationBore(&raw, q)
			}
		}
		if b.RimFillet != nil {
			count++
			if count == 1 {
				q := *b.RimFillet
				q.LengthUnit = m.LengthUnit
				e = WriteWrappedRimFillet(&raw, q)
			}
		}
		if count != 1 {
			return geom.ErrInvalid
		}
		if e != nil {
			return e
		}
		f, e := Parse(&raw, Limits{})
		if e != nil {
			return e
		}
		root := int64(0)
		for _, id := range f.Order {
			_, ordinary := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
			_, void := simple(f, id, "BREP_WITH_VOIDS", 3)
			if ordinary == nil || void == nil {
				if root != 0 {
					return geom.ErrInvalid
				}
				root = id
			}
		}
		if root == 0 {
			return geom.ErrInvalid
		}
		files = append(files, f)
		roots = append(roots, root)
	}
	var out bytes.Buffer
	e := writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := []int64{}
		for i, f := range files {
			ids = append(ids, copySplineGeometry(emit, f, []int64{roots[i]})[0])
		}
		return ids
	})
	if e != nil {
		return e
	}
	if _, e = ReadWrappedMaterials(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}
