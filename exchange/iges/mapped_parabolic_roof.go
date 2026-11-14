package iges

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// MappedParabolicRoofSolid preserves original source IDs and list positions,
// indexed by the documented canonical geometric roles. AuthoredFaceOrder retains
// the actual source shell order. No native geometry is inferred from record order.
type MappedParabolicRoofSolid struct {
	ParabolicRoofSolid
	SourceVertexIndices [8]int
	SourceEdgeIndices   [12]int
	SourceUseIndices    [6][4]int // canonical coedge role to original one-based loop position
	SourceLoopIDs       [6]int
	AuthoredFaceOrder   [6]int
}

// ReadMappedParabolicRoofSolid accepts independent directory, vertex-list,
// edge-list and shell-face ordering, and cyclic loop starts. Source geometry and
// directed edge/chart parameterizations must remain exactly the parabolic family;
// arbitrary reparameterization or reversed support charts remain unsupported.
func ReadMappedParabolicRoofSolid(ctx context.Context, input io.Reader, l Limits) (MappedParabolicRoofSolid, error) {
	fail := func() (MappedParabolicRoofSolid, error) { return MappedParabolicRoofSolid{}, roofFailure() }
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return MappedParabolicRoofSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 58)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxControlPoints = min(l.MaxControlPoints, 104)
	doc, e := readTopologySource(input, l, true)
	if e != nil {
		return MappedParabolicRoofSolid{}, e
	}
	if len(doc.entities) != 58 || l.MaxControlPoints < 104 {
		return fail()
	}
	// Appearance/level references are outside this material-only dialect.
	for _, line := range bytes.Split(doc.bytes, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			seq, err := field(string(line[73:]))
			if err != nil {
				return fail()
			}
			indexes := []int{1, 2}
			if seq%2 == 1 {
				kind, err := field(string(line[:8]))
				if err != nil {
					return fail()
				}
				status, err := field(string(line[64:72]))
				if err != nil {
					return fail()
				}
				want := 10000
				if kind == 186 {
					want = 0
				}
				if kind == 504 {
					want = 10001
				}
				if status != want {
					return fail()
				}
				indexes = []int{3, 4}
			}
			for _, j := range indexes {
				v, err := field(string(line[j*8 : j*8+8]))
				if err != nil || v != 0 {
					return fail()
				}
			}
		}
	}
	records := map[int]topologyRecord{}
	kinds := map[int][]int{}
	geometry := map[int]Entity{}
	metadata := map[int]CylinderDatumMetadata{}
	used := map[int]bool{}
	for _, r := range doc.entities {
		records[r.id] = r
		kinds[r.kind] = append(kinds[r.kind], r.id)
		metadata[r.id] = CylinderDatumMetadata{r.label, r.subscript}
		if e = compositeSourceNumbers(110, r.values); e != nil {
			return fail()
		}
		if r.kind == 126 || r.kind == 128 {
			en, e := decode(r.kind, r.values, 104)
			if e != nil || en.sourceWeightsRounded {
				return fail()
			}
			if e = en.validate(); e != nil {
				return fail()
			}
			geometry[r.id] = en
		}
	}
	for k, n := range map[int]int{186: 1, 514: 1, 510: 6, 508: 6, 502: 1, 504: 1, 128: 6, 126: 36} {
		if len(kinds[k]) != n {
			return fail()
		}
	}
	take := func(id, kind int) (topologyRecord, bool) {
		r, ok := records[id]
		if !ok || r.kind != kind || used[id] {
			return topologyRecord{}, false
		}
		used[id] = true
		return r, true
	}
	bodyID, shellID, vertexID, edgeID := kinds[186][0], kinds[514][0], kinds[502][0], kinds[504][0]
	r, _ := take(bodyID, 186)
	v, e := topologyInts(r, 4)
	if e != nil || v[1] != shellID || v[2] != 1 || v[3] != 0 {
		return fail()
	}
	r, _ = take(shellID, 514)
	sv, e := topologyInts(r, 14)
	if e != nil || sv[1] != 6 {
		return fail()
	}
	var out MappedParabolicRoofSolid
	shellFaces := map[int]bool{}
	for i := 0; i < 6; i++ {
		id := sv[2+2*i]
		if sv[3+2*i] != 1 || shellFaces[id] {
			return fail()
		}
		shellFaces[id] = true
		out.AuthoredFaceOrder[i] = id
	}
	r, _ = take(vertexID, 502)
	if len(r.values) < 26 || noPointers(r.values[26:]) != nil {
		return fail()
	}
	kind, e := intValue(r.values[0])
	if e != nil || kind != 502 {
		return fail()
	}
	n, e := intValue(r.values[1])
	if e != nil || n != 8 {
		return fail()
	}
	var vertices [8]geom.Vec3
	for i := range vertices {
		var c [3]float64
		for j := range c {
			c[j], e = realValue(r.values[2+3*i+j])
			if e != nil {
				return fail()
			}
		}
		vertices[i] = geom.Vec3{X: c[0], Y: c[1], Z: c[2]}
	}
	lo, hi := vertices[0], vertices[0]
	for _, p := range vertices {
		lo.X = min(lo.X, p.X)
		lo.Y = min(lo.Y, p.Y)
		lo.Z = min(lo.Z, p.Z)
		hi.X = max(hi.X, p.X)
		hi.Y = max(hi.Y, p.Y)
		hi.Z = max(hi.Z, p.Z)
	}
	roofBase := hi.Z
	for _, p := range vertices {
		if p.Z > lo.Z {
			roofBase = min(roofBase, p.Z)
		}
	}
	difference := func(a, b float64) (float64, error) {
		x, exact := new(big.Rat).Sub(tetraRat(a), tetraRat(b)).Float64()
		if !exact {
			return 0, geom.ErrUnrepresentable
		}
		return x, nil
	}
	d := ParabolicRoofDescriptor{Origin: lo}
	d.Width, e = difference(hi.X, lo.X)
	if e != nil {
		return fail()
	}
	d.Depth, e = difference(hi.Y, lo.Y)
	if e != nil {
		return fail()
	}
	d.Height, e = difference(roofBase, lo.Z)
	if e != nil {
		return fail()
	}
	d.Rise, e = difference(hi.Z, roofBase)
	if e != nil {
		return fail()
	}
	expected, roles, e := roofAuthor(d)
	if e != nil {
		return fail()
	}
	expectedBytes, e := expected.file(doc.unit)
	if e != nil {
		return fail()
	}
	canon, e := readTopologySource(bytes.NewReader(expectedBytes), Limits{}, true)
	if e != nil {
		return fail()
	}
	cr := map[int]topologyRecord{}
	for _, r := range canon.entities {
		cr[r.id] = r
	}
	sourceForCanonical := map[int]int{roles.solid: bodyID, roles.shell: shellID, roles.vertices: vertexID, roles.edges: edgeID}
	var vertexRole [8]int
	matchedVertices := map[int]bool{}
	for i, p := range vertices {
		found := -1
		for j := 0; j < 8; j++ {
			var c [3]float64
			for k := range c {
				c[k], _ = realValue(cr[roles.vertices].values[2+3*j+k])
			}
			if p == (geom.Vec3{X: c[0], Y: c[1], Z: c[2]}) {
				found = j
			}
		}
		if found < 0 || matchedVertices[found] {
			return fail()
		}
		matchedVertices[found] = true
		vertexRole[i] = found
		out.SourceVertexIndices[found] = i + 1
	}
	sameGeometry := func(source, canonical int) bool {
		en, ok := geometry[source]
		if !ok {
			return false
		}
		_, actual := encode(en)
		return actual == expected.records[(canonical-1)/2].data
	}
	r, _ = take(edgeID, 504)
	ev, e := topologyInts(r, 62)
	if e != nil || ev[1] != 12 {
		return fail()
	}
	ce, e := topologyInts(cr[roles.edges], 62)
	if e != nil {
		return fail()
	}
	var edgeRole [12]int
	matchedEdges := map[int]bool{}
	for i := 0; i < 12; i++ {
		a := ev[2+5*i : 7+5*i]
		if a[1] != vertexID || a[3] != vertexID || a[2] < 1 || a[2] > 8 || a[4] < 1 || a[4] > 8 {
			return fail()
		}
		start, end := vertexRole[a[2]-1], vertexRole[a[4]-1]
		role := -1
		for j := 0; j < 12; j++ {
			b := ce[2+5*j : 7+5*j]
			if start == b[2]-1 && end == b[4]-1 {
				role = j
			}
		}
		if role < 0 || matchedEdges[role] {
			return fail()
		}
		if _, ok := take(a[0], 126); !ok || !sameGeometry(a[0], roles.models[role]) {
			return fail()
		}
		matchedEdges[role] = true
		edgeRole[i] = role
		out.SourceEdgeIndices[role] = i + 1
		sourceForCanonical[roles.models[role]] = a[0]
	}
	matchedFaces := map[int]bool{}
	for _, fid := range out.AuthoredFaceOrder {
		r, ok := take(fid, 510)
		if !ok {
			return fail()
		}
		fv, e := topologyInts(r, 5)
		if e != nil || fv[2] != 1 || fv[3] != 1 {
			return fail()
		}
		sid, lid := fv[1], fv[4]
		if _, ok = take(sid, 128); !ok {
			return fail()
		}
		role := -1
		for j, cid := range roles.surfaces {
			if sameGeometry(sid, cid) {
				if role != -1 {
					return fail()
				}
				role = j
			}
		}
		if role < 0 || matchedFaces[role] {
			return fail()
		}
		matchedFaces[role] = true
		r, ok = take(lid, 508)
		if !ok {
			return fail()
		}
		lv, e := topologyInts(r, 30)
		if e != nil || lv[1] != 4 {
			return fail()
		}
		canonicalLoop := roles.faces[role] - 2
		cv, e := topologyInts(cr[canonicalLoop], 30)
		if e != nil {
			return fail()
		}
		var positions [4]int
		for j := 0; j < 4; j++ {
			a := lv[2+7*j : 9+7*j]
			if a[0] != 0 || a[1] != edgeID || a[2] < 1 || a[2] > 12 || a[4] != 1 || a[5] != 0 {
				return fail()
			}
			er := edgeRole[a[2]-1]
			pos := -1
			for k := 0; k < 4; k++ {
				b := cv[2+7*k : 9+7*k]
				if er == b[2]-1 && a[3] == b[3] {
					pos = k
				}
			}
			if pos < 0 {
				return fail()
			}
			positions[j] = pos
			out.SourceUseIndices[role][pos] = j + 1
			if _, ok = take(a[6], 126); !ok || !sameGeometry(a[6], roles.pc[role][pos]) {
				return fail()
			}
			sourceForCanonical[roles.pc[role][pos]] = a[6]
		}
		for j := 0; j < 4; j++ {
			if positions[(j+1)%4] != (positions[j]+1)%4 {
				return fail()
			}
		}
		sourceForCanonical[roles.faces[role]] = fid
		sourceForCanonical[roles.surfaces[role]] = sid
		sourceForCanonical[canonicalLoop] = lid
		out.SourceLoopIDs[role] = lid
	}
	if len(used) != 58 || len(sourceForCanonical) != 58 {
		return fail()
	}
	// Complete directed source correspondence has now been established. The
	// canonical material proof is valid for these identical original coefficients;
	// returned geometry and provenance remain keyed by the actual source IDs.
	native, e := ReadParabolicRoofSolid(ctx, bytes.NewReader(expectedBytes), l)
	if e != nil {
		return MappedParabolicRoofSolid{}, e
	}
	native.Geometry = geometry
	native.Metadata = metadata
	native.SourceSolidID = bodyID
	native.SourceShellID = shellID
	native.SourceVertexListID = vertexID
	native.SourceEdgeListID = edgeID
	for i, id := range roles.faces {
		native.SourceFaceIDs[i] = sourceForCanonical[id]
		native.SourceSurfaceIDs[i] = sourceForCanonical[roles.surfaces[i]]
		for j, id := range roles.pc[i] {
			native.SourcePCurveIDs[i][j] = sourceForCanonical[id]
		}
	}
	for i, id := range roles.models {
		native.SourceModelCurveIDs[i] = sourceForCanonical[id]
	}
	out.ParabolicRoofSolid = native
	return out, nil
}
