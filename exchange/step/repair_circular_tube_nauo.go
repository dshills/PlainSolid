package step

import (
	"bytes"
	"io"
	"math/big"
	"plainsolid/brep/parametric"
)

// RepairedCircularTubeNAUO is a newly reconstructed concentric through-tube.
// Walls contains original outer/inner source IDs and separately bounded seam
// repairs. Its inner wall's evidence uses positive cylindrical orientation for
// calculation only; the resulting material has an inward inner wall. Neither
// this result nor the two reports asserts source-identical finite trimming.
type RepairedCircularTubeNAUO struct {
	Body                  TubeBody
	Solid                 parametric.Solid
	LengthUnit            float64
	Walls                 [2]CircularTubeNAUOWallEvidence
	SourceBodyID, ShellID int64
	CapFaceIDs            [2]int64 // lower, upper original annular faces
}

// CircularTubeNAUOWallEvidence preserves the original source wall orientation
// and master pcurve selection. Source IDs and all residuals are original;
// bounds use the wall's positive cylinder as a geometric proof factor only.
type CircularTubeNAUOWallEvidence struct {
	Source                CircularNAUOInspection
	Bounds                CircularNAUORepairBounds
	SourceFaceSense       bool
	RimMasterPCurveIDs    [2]int64 // lower, upper original selected owner
	ModelSeamMagnitude    *big.Rat // original positive LINE magnitude
	ModelSeamParameterEnd *big.Rat // repaired endpoint selection height/magnitude; not authored trim
}

// circularTubeLoop normalizes FACE_BOUND orientation in a private proof graph.
// It optionally reverses the complete directed loop when factoring an inner
// material boundary into a positive cylinder. All original supports, vertices,
// edge senses, model and pcurve coefficients remain untouched.
func circularTubeLoop(f *File, bound int64, reverse bool, seen map[int64]bool) ([]analyticUse, error) {
	fail := func() ([]analyticUse, error) { return nil, mappedPlanarFailure() }
	if seen[bound] {
		return fail()
	}
	seen[bound] = true
	b, e := simple(f, bound, "FACE_BOUND", 3)
	if e != nil {
		return fail()
	}
	sense, e := boolean(b.Args[2])
	if e != nil {
		return fail()
	}
	loopID, e := ref(b.Args[1])
	if e != nil || seen[loopID] {
		return fail()
	}
	seen[loopID] = true
	loop, e := simple(f, loopID, "EDGE_LOOP", 2)
	if e != nil {
		return fail()
	}
	ids, e := listRefs(loop.Args[1], 1)
	if e != nil || (len(ids) != 1 && len(ids) != 4) {
		return fail()
	}
	flip := !sense != reverse
	uses := make([]analyticUse, len(ids))
	items := make([]Value, len(ids))
	for i, id := range ids {
		if seen[id] {
			return fail()
		}
		seen[id] = true
		u, e := simple(f, id, "ORIENTED_EDGE", 5)
		if e != nil || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived {
			return fail()
		}
		edge, e := ref(u.Args[3])
		if e != nil {
			return fail()
		}
		s, e := boolean(u.Args[4])
		if e != nil {
			return fail()
		}
		j := i
		if flip {
			j = len(ids) - 1 - i
			s = !s
		}
		u.Args[4] = Value{Kind: Enumeration, Text: "F"}
		if s {
			u.Args[4].Text = "T"
		}
		ent := f.Entities[id]
		ent.Records[0] = u
		f.Entities[id] = ent
		uses[j] = analyticUse{edge, s}
		items[j] = Value{Kind: Reference, Ref: id}
	}
	loop.Args[1] = Value{Kind: Aggregate, Items: items}
	ent := f.Entities[loopID]
	ent.Records[0] = loop
	f.Entities[loopID] = ent
	b.Args[2] = Value{Kind: Enumeration, Text: "T"}
	ent = f.Entities[bound]
	ent.Records[0] = b
	f.Entities[bound] = ent
	return uses, nil
}

// RepairCircularTubeNAUOProduct is opt-in reconstruction of a narrowly bounded
// original four-face annular cylinder product. Original outer/inner supports
// must have identical exact signed-cardinal frames and positive shared height,
// strictly ordered radii, two shared concentric annular caps, opposite material
// wall senses, and exclusive closed rim/seam ownership. Every source vertex,
// model seam and owner seam repair satisfies CircularNAUORepairOptions; the
// tolerance is additionally at most one millionth of the wall thickness.
// It does not accept other tube families, infer finite trims, or relax any
// exact reader. Errors return no partially reconstructed result.
func RepairCircularTubeNAUOProduct(r io.Reader, l Limits, productID int64, o CircularNAUORepairOptions) (RepairedCircularTubeNAUO, error) {
	fail := func() (RepairedCircularTubeNAUO, error) {
		return RepairedCircularTubeNAUO{}, mappedPlanarFailure()
	}
	lim, e := l.defaults()
	if e != nil || r == nil || lim.MaxFaces < 16 || lim.MaxVertices < 16 || lim.MaxEdgeUses < 64 {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, lim.MaxBytes+1))
	if e != nil {
		return RepairedCircularTubeNAUO{}, e
	}
	if int64(len(raw)) > lim.MaxBytes {
		return fail()
	}
	a, e := ReadAssembly(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	f := a.Source
	if len(f.Entities) > 8192 || len(a.Products) > 8 || len(a.Occurrences) > 32 {
		return fail()
	}
	ids, advanced, e := a.productItems(productID)
	if e != nil || len(ids) != 1 || !advanced[0] {
		return fail()
	}
	// Cross-product ownership is checked on the complete original shell,
	// before factoring cap loops or complementing any orientation.
	closure, e := nauoClosure(f, ids, lim.MaxDepth)
	if e != nil {
		return fail()
	}
	for _, p := range a.Products {
		if p.DefinitionID == productID {
			continue
		}
		otherIDs, _, e := a.productItems(p.DefinitionID)
		if e != nil {
			return fail()
		}
		other, e := nauoClosure(f, otherIDs, lim.MaxDepth)
		if e != nil {
			return fail()
		}
		for id := range other {
			if !closure[id] {
				continue
			}
			for _, rec := range f.Entities[id].Records {
				if planarSplineOwnedType(rec.Type) || rec.Type == "SEAM_CURVE" || rec.Type == "CARTESIAN_POINT" || rec.Type == "DIRECTION" || rec.Type == "VECTOR" || rec.Type == "AXIS2_PLACEMENT_3D" {
					return fail()
				}
			}
		}
	}
	body, e := simple(f, ids[0], "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	shellID, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	faces, e := listRefs(shell.Args[1], 4)
	if e != nil || len(faces) != 4 {
		return fail()
	}
	var walls [2]int64
	caps := []int64{}
	unique := map[int64]bool{}
	for _, id := range faces {
		if unique[id] {
			return fail()
		}
		unique[id] = true
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		ent := f.Entities[sid]
		if len(ent.Records) != 1 {
			return fail()
		}
		switch ent.Records[0].Type {
		case "CYLINDRICAL_SURFACE":
			sense, e := boolean(face.Args[3])
			if e != nil {
				return fail()
			}
			i := 0
			if !sense {
				i = 1
			}
			if walls[i] != 0 {
				return fail()
			}
			walls[i] = id
		case "PLANE":
			caps = append(caps, id)
		default:
			return fail()
		}
	}
	if walls[0] == 0 || walls[1] == 0 || len(caps) != 2 {
		return fail()
	}
	// Normalize the two original material wall loops, recording the unique rim
	// owner. A false inner face is complemented solely in this proof factor.
	incoming := map[int64]int{}
	var count func(Value)
	count = func(v Value) {
		if v.Kind == Reference {
			incoming[v.Ref]++
		}
		for _, x := range v.Items {
			count(x)
		}
	}
	for _, ent := range f.Entities {
		for _, rec := range ent.Records {
			for _, v := range rec.Args {
				count(v)
			}
		}
	}
	magnitudes := [2]*big.Rat{}
	seen := map[int64]bool{}
	rimOwner := map[int64]int{}

	for i, id := range walls {
		face, _ := simple(f, id, "ADVANCED_FACE", 4)
		bs, e := listRefs(face.Args[1], 1)
		if e != nil || len(bs) != 1 {
			return fail()
		}

		uses, e := circularTubeLoop(f, bs[0], i == 1, seen)
		if e != nil || len(uses) != 4 {
			return fail()
		}
		counts := map[int64]int{}
		for _, u := range uses {
			counts[u.id]++
		}
		if len(counts) != 3 {
			return fail()
		}
		for edge, n := range counts {
			if n == 1 {
				if _, ok := rimOwner[edge]; ok {
					return fail()
				}
				rimOwner[edge] = i
			} else if n != 2 {
				return fail()
			} else {
				_, _, geometry, e := analyticEdge(f, edge)
				if e != nil {
					return fail()
				}
				seam, e := simple(f, geometry, "SEAM_CURVE", 4)
				if e != nil {
					return fail()
				}
				lineID, e := ref(seam.Args[1])
				if e != nil || incoming[lineID] != 1 {
					return fail()
				}
				line, e := simple(f, lineID, "LINE", 3)
				if e != nil {
					return fail()
				}
				vectorID, e := ref(line.Args[2])
				if e != nil || incoming[vectorID] != 1 {
					return fail()
				}
				vector, e := simple(f, vectorID, "VECTOR", 3)
				if e != nil {
					return fail()
				}
				magnitude, e := liftSourceRat(vector.Args[2])
				if e != nil || magnitude.Sign() <= 0 {
					return fail()
				}
				magnitudes[i] = magnitude
				// Exact change of parameter s=m*t only. The original finite trim is
				// unknown; the explicit reconstruction selects t in [0,height/m].
				vector.Args[2] = Value{Kind: Number, Text: "1."}
				ent := f.Entities[vectorID]
				ent.Records[0] = vector
				f.Entities[vectorID] = ent
			}
		}
	}
	if len(rimOwner) != 4 {
		return fail()
	}
	// Cap loop ownership must be one distinct complete rim from each wall.
	capBounds := [2][2]int64{}
	usedRims := map[int64]bool{}
	for c, id := range caps {
		face, _ := simple(f, id, "ADVANCED_FACE", 4)
		bs, e := listRefs(face.Args[1], 2)
		if e != nil || len(bs) != 2 {
			return fail()
		}
		for _, b := range bs {
			// Discover the referenced edge before choosing the inner complement.
			br, e := simple(f, b, "FACE_BOUND", 3)
			if e != nil {
				return fail()
			}
			lr, e := simple(f, br.Args[1].Ref, "EDGE_LOOP", 2)
			if e != nil {
				return fail()
			}
			us, e := listRefs(lr.Args[1], 1)
			if e != nil || len(us) != 1 {
				return fail()
			}
			ur, e := simple(f, us[0], "ORIENTED_EDGE", 5)
			if e != nil {
				return fail()
			}
			edge, e := ref(ur.Args[3])
			if e != nil {
				return fail()
			}
			owner, ok := rimOwner[edge]
			if !ok || usedRims[edge] || capBounds[c][owner] != 0 {
				return fail()
			}
			usedRims[edge] = true
			capBounds[c][owner] = b
			if _, e = circularTubeLoop(f, b, owner == 1, seen); e != nil {
				return fail()
			}
		}
	}
	// Each proof slice retains original IDs and all original geometric numbers.
	// Only annular-cap subset and oriented inner complement are selected; the
	// existing strict cylinder proof checks continuous support lifts and closed
	// coedge incidence independently for each factor.
	normalized, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return fail()
	}
	var result RepairedCircularTubeNAUO
	var repaired [2]RepairedCircularNAUO
	master := map[int64]int64{}
	for i, wall := range walls {
		q, e := Parse(bytes.NewReader(normalized), l)
		if e != nil {
			return fail()
		}
		sh := q.Entities[shellID]
		sh.Records[0].Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: wall}, {Kind: Reference, Ref: caps[0]}, {Kind: Reference, Ref: caps[1]}}}
		q.Entities[shellID] = sh
		w := q.Entities[wall]
		w.Records[0].Args[3] = Value{Kind: Enumeration, Text: "T"}
		q.Entities[wall] = w
		for c, id := range caps {
			v := q.Entities[id]
			v.Records[0].Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: capBounds[c][i]}}}
			q.Entities[id] = v
		}
		// Preserve original master ownership. The strict geometric factor proves
		// BOTH full analytic lifts equal the same model circle, so association
		// order is irrelevant to that support identity. This does not establish
		// the original master owner's finite trim or parameter-selection semantics.
		for edge, owner := range rimOwner {
			if owner != i {
				continue
			}
			_, _, g, e := analyticEdge(q, edge)
			if e != nil {
				return fail()
			}
			sc, e := simple(q, g, "SURFACE_CURVE", 4)
			if e != nil || sc.Args[3].Kind != Enumeration || sc.Args[3].Text != "PCURVE_S1" {
				return fail()
			}
			ps, e := listRefs(sc.Args[2], 2)
			if e != nil || len(ps) != 2 {
				return fail()
			}
			master[edge] = ps[0]
			first, e := simple(q, ps[0], "PCURVE", 3)
			if e != nil {
				return fail()
			}
			wallRec, _ := simple(q, wall, "ADVANCED_FACE", 4)
			if first.Args[1].Ref != wallRec.Args[2].Ref {
				sc.Args[2].Items[0], sc.Args[2].Items[1] = sc.Args[2].Items[1], sc.Args[2].Items[0]
				ent := q.Entities[g]
				ent.Records[0] = sc
				q.Entities[g] = ent
			}
		}
		data, e := mappedPlanarSourceBytes(q)
		if e != nil {
			return fail()
		}
		repaired[i], e = RepairCircularNAUOProduct(bytes.NewReader(data), l, productID, o)
		if e != nil {
			return fail()
		}
	}
	outer, inner := repaired[0], repaired[1]
	for i, w := range repaired {
		result.Walls[i] = CircularTubeNAUOWallEvidence{Source: w.Source, Bounds: w.Bounds, SourceFaceSense: i == 0, ModelSeamMagnitude: new(big.Rat).Set(magnitudes[i]), ModelSeamParameterEnd: new(big.Rat).Quo(new(big.Rat).SetFloat64(w.Body.Height), magnitudes[i]), RimMasterPCurveIDs: [2]int64{master[w.Source.Rims[0].EdgeID], master[w.Source.Rims[1].EdgeID]}}
	}
	if outer.Body.Frame != inner.Body.Frame || outer.Body.Height != inner.Body.Height || outer.Body.Radius <= inner.Body.Radius || outer.LengthUnit != inner.LengthUnit {
		return fail()
	}
	for i := 0; i < 2; i++ {
		if outer.Source.Rims[i].CapFaceID != inner.Source.Rims[i].CapFaceID {
			return fail()
		}
		result.CapFaceIDs[i] = outer.Source.Rims[i].CapFaceID
	}
	thickness := new(big.Rat).Sub(new(big.Rat).SetFloat64(outer.Body.Radius), new(big.Rat).SetFloat64(inner.Body.Radius))
	if outer.Bounds.Tolerance.Cmp(new(big.Rat).Quo(thickness, big.NewRat(1000000, 1))) > 0 {
		return fail()
	}
	tf, exact := thickness.Float64()
	if !exact {
		return fail()
	}
	solid, e := parametric.Tube(outer.Body.Frame, outer.Body.Radius, outer.Body.Height, tf)
	if e != nil {
		return fail()
	}
	result.Body = TubeBody{Name: outer.Body.Name, Frame: outer.Body.Frame, OuterRadius: outer.Body.Radius, InnerRadius: inner.Body.Radius, Height: outer.Body.Height}
	result.Solid = solid
	result.LengthUnit = outer.LengthUnit
	result.SourceBodyID = ids[0]
	result.ShellID = shellID
	return result, nil
}
