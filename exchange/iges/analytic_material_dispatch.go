package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// AnalyticMaterialFamily identifies an authenticated registered material graph,
// not a general IGES entity classification.
type AnalyticMaterialFamily string

const (
	AnalyticSphere               AnalyticMaterialFamily = "sphere"
	AnalyticTorus                AnalyticMaterialFamily = "ring-torus"
	AnalyticCone                 AnalyticMaterialFamily = "conical45-frustum"
	AnalyticSphereCavity         AnalyticMaterialFamily = "spherical-cavity"
	AnalyticConeCavity           AnalyticMaterialFamily = "conical45-cavity"
	AnalyticCylinderSphereCavity AnalyticMaterialFamily = "cylinder-sphere-cavity"
	AnalyticConeCylinderThrough  AnalyticMaterialFamily = "conical45-cylinder-through"
)

// AnalyticMaterial contains exactly one independently re-proved typed result.
// Its private variant prevents forged tags or mixed payloads. SourceRoles and
// NativeFaceSourceIDs expose copied ancestry; source export retains all metadata.
// Sphere variants retain their documented bounded native angular evaluation.
type AnalyticMaterial struct {
	sphere               *ReindexedSphericalMaterial
	torus                *ReindexedToroidalMaterial
	cone                 *ReindexedConical45FrustumMaterial
	sphereCavity         *ReindexedSphericalCavityMaterial
	coneCavity           *ReindexedConical45CavityMaterial
	cylinderSphereCavity *ReindexedCylinderSphereCavityMaterial
	coneCylinderThrough  *ReindexedConical45CylinderThroughMaterial
}

// Family identifies the authenticated variant; the zero value is invalid.
func (m AnalyticMaterial) Family() (AnalyticMaterialFamily, error) {
	var family AnalyticMaterialFamily
	count := 0
	if m.sphere != nil {
		family = AnalyticSphere
		count++
	}
	if m.torus != nil {
		family = AnalyticTorus
		count++
	}
	if m.cone != nil {
		family = AnalyticCone
		count++
	}
	if m.sphereCavity != nil {
		family = AnalyticSphereCavity
		count++
	}
	if m.coneCavity != nil {
		family = AnalyticConeCavity
		count++
	}
	if m.cylinderSphereCavity != nil {
		family = AnalyticCylinderSphereCavity
		count++
	}
	if m.coneCylinderThrough != nil {
		family = AnalyticConeCylinderThrough
		count++
	}
	if count != 1 {
		return "", geom.ErrInvalid
	}
	return family, nil
}

// Solid returns the selected native material; it does not reauthenticate caller edits.
// Use WriteAnalyticMaterialSource to reprove source binding before export.
func (m AnalyticMaterial) Solid() (trimmed.Solid, error) {
	family, e := m.Family()
	if e != nil {
		return trimmed.Solid{}, e
	}
	switch family {
	case AnalyticSphere:
		return m.sphere.Solid(), nil
	case AnalyticTorus:
		return m.torus.Solid(), nil
	case AnalyticCone:
		return m.cone.Solid(), nil
	case AnalyticSphereCavity:
		return m.sphereCavity.Solid(), nil
	case AnalyticConeCavity:
		return m.coneCavity.Solid(), nil
	case AnalyticCylinderSphereCavity:
		return m.cylinderSphereCavity.Solid(), nil
	case AnalyticConeCylinderThrough:
		return m.coneCylinderThrough.Solid(), nil
	}
	return trimmed.Solid{}, geom.ErrInvalid
}

// ReadAnalyticMaterial routes a bounded structural signature to exactly one
// existing strict codec. A matching signature alone is never acceptance.
// Unknown, combined, orphaned or invalid graphs return no partial result.
func ReadAnalyticMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (AnalyticMaterial, error) {
	bad := func() (AnalyticMaterial, error) {
		return AnalyticMaterial{}, fmt.Errorf("iges: unsupported registered analytic material signature")
	}
	if ctx == nil || r == nil || !finite(tol) || tol <= 0 {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return AnalyticMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 40)
	l.MaxValues = min(l.MaxValues, 2048)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return AnalyticMaterial{}, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return bad()
	}
	var signature [8]int
	entities, records, directory := 0, 0, 0
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if e := ctx.Err(); e != nil {
			return AnalyticMaterial{}, e
		}
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) == 0 {
			continue
		}
		records++
		if records > l.MaxRecords || len(line) != 80 {
			return bad()
		}
		if line[72] != 'D' {
			continue
		}
		directory++
		if directory%2 == 0 {
			continue
		}
		entities++
		if entities > l.MaxEntities {
			return bad()
		}
		kind, e := field(string(line[:8]))
		if e != nil {
			return bad()
		}
		for i, k := range []int{186, 514, 510, 190, 192, 194, 196, 198} {
			if kind == k {
				signature[i]++
			}
		}
	}
	if directory%2 != 0 {
		return bad()
	}
	switch signature {
	case [8]int{1, 1, 1, 0, 0, 0, 1, 0}:
		p, e := ReadReindexedSphericalMaterial(ctx, bytes.NewReader(raw), l, tol)
		if e != nil {
			return AnalyticMaterial{}, e
		}
		return AnalyticMaterial{sphere: &p}, nil
	case [8]int{1, 1, 1, 0, 0, 0, 0, 1}:
		p, e := ReadReindexedToroidalMaterial(ctx, bytes.NewReader(raw), l, tol)
		if e != nil {
			return AnalyticMaterial{}, e
		}
		return AnalyticMaterial{torus: &p}, nil
	case [8]int{1, 1, 3, 2, 0, 1, 0, 0}:
		p, e := ReadReindexedConical45FrustumMaterial(ctx, bytes.NewReader(raw), l, tol)
		if e != nil {
			return AnalyticMaterial{}, e
		}
		return AnalyticMaterial{cone: &p}, nil
	case [8]int{1, 2, 2, 0, 0, 0, 2, 0}:
		p, e := ReadReindexedSphericalCavityMaterial(ctx, bytes.NewReader(raw), l, tol)
		if e != nil {
			return AnalyticMaterial{}, e
		}
		return AnalyticMaterial{sphereCavity: &p}, nil
	case [8]int{1, 2, 6, 4, 0, 2, 0, 0}:
		p, e := ReadReindexedConical45CavityMaterial(ctx, bytes.NewReader(raw), l, tol)
		if e != nil {
			return AnalyticMaterial{}, e
		}
		return AnalyticMaterial{coneCavity: &p}, nil
	case [8]int{1, 2, 4, 2, 1, 0, 1, 0}:
		p, e := ReadReindexedCylinderSphereCavityMaterial(ctx, bytes.NewReader(raw), l, tol)
		if e != nil {
			return AnalyticMaterial{}, e
		}
		return AnalyticMaterial{cylinderSphereCavity: &p}, nil
	case [8]int{1, 1, 4, 2, 1, 1, 0, 0}:
		p, e := ReadReindexedConical45CylinderThroughMaterial(ctx, bytes.NewReader(raw), l, tol)
		if e != nil {
			return AnalyticMaterial{}, e
		}
		return AnalyticMaterial{coneCylinderThrough: &p}, nil
	}
	return bad()
}

// WriteAnalyticMaterialSource preserves original bytes after full variant
// reauthentication; it never synthesizes or repairs an unrelated graph.
func WriteAnalyticMaterialSource(ctx context.Context, w io.Writer, m AnalyticMaterial) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	family, e := m.Family()
	if e != nil {
		return e
	}
	switch family {
	case AnalyticSphere:
		return WriteReindexedSphericalSource(ctx, w, *m.sphere)
	case AnalyticTorus:
		return WriteReindexedToroidalSource(ctx, w, *m.torus)
	case AnalyticCone:
		return WriteReindexedConical45FrustumSource(ctx, w, *m.cone)
	case AnalyticSphereCavity:
		return WriteReindexedSphericalCavitySource(ctx, w, *m.sphereCavity)
	case AnalyticConeCavity:
		return WriteReindexedConical45CavitySource(ctx, w, *m.coneCavity)
	case AnalyticCylinderSphereCavity:
		return WriteReindexedCylinderSphereCavitySource(ctx, w, *m.cylinderSphereCavity)
	case AnalyticConeCylinderThrough:
		return WriteReindexedConical45CylinderThroughSource(ctx, w, *m.coneCylinderThrough)
	}
	return geom.ErrInvalid
}

// SourceRoles returns a caller-owned canonical-role to original-record map.
func (m AnalyticMaterial) SourceRoles() map[int]int {
	out := map[int]int{}
	family, e := m.Family()
	if e != nil {
		return out
	}
	var roles map[int]int
	switch family {
	case AnalyticSphere:
		roles = m.sphere.SourceForCanonical
	case AnalyticTorus:
		roles = m.torus.SourceForCanonical
	case AnalyticCone:
		roles = m.cone.SourceForCanonical
	case AnalyticSphereCavity:
		roles = m.sphereCavity.SourceForCanonical
	case AnalyticConeCavity:
		roles = m.coneCavity.SourceForCanonical
	case AnalyticCylinderSphereCavity:
		roles = m.cylinderSphereCavity.SourceForCanonical
	case AnalyticConeCylinderThrough:
		roles = m.coneCylinderThrough.SourceForCanonical
	}
	for k, v := range roles {
		out[k] = v
	}
	return out
}

// NativeFaceSourceIDs returns a caller-owned original face ancestry list.
func (m AnalyticMaterial) NativeFaceSourceIDs() []int {
	family, e := m.Family()
	if e != nil {
		return nil
	}
	switch family {
	case AnalyticSphere:
		return append([]int(nil), m.sphere.NativeFaceSourceIDs[:]...)
	case AnalyticTorus:
		return append([]int(nil), m.torus.NativeFaceSourceIDs[:]...)
	case AnalyticCone:
		return append([]int(nil), m.cone.NativeFaceSourceIDs[:]...)
	case AnalyticSphereCavity:
		return append([]int(nil), m.sphereCavity.NativeFaceSourceIDs[:]...)
	case AnalyticConeCavity:
		return append([]int(nil), m.coneCavity.NativeFaceSourceIDs[:]...)
	case AnalyticCylinderSphereCavity:
		return append([]int(nil), m.cylinderSphereCavity.NativeFaceSourceIDs[:]...)
	case AnalyticConeCylinderThrough:
		return append([]int(nil), m.coneCylinderThrough.NativeFaceSourceIDs[:]...)
	}
	return nil
}

// LengthUnit returns the authenticated meters per model unit without rescaling geometry.
func (m AnalyticMaterial) LengthUnit() (float64, error) {
	family, e := m.Family()
	if e != nil {
		return 0, e
	}
	switch family {
	case AnalyticSphere:
		p := m.sphere
		if p.Local != nil {
			return p.Local.LengthUnit, nil
		}
		if p.Placed != nil {
			return p.Placed.Local.LengthUnit, nil
		}
	case AnalyticTorus:
		p := m.torus
		if p.Local != nil {
			return p.Local.LengthUnit, nil
		}
		if p.Placed != nil {
			return p.Placed.Local.LengthUnit, nil
		}
	case AnalyticCone:
		p := m.cone
		if p.Local != nil {
			return p.Local.LengthUnit, nil
		}
		if p.Placed != nil {
			return p.Placed.Local.LengthUnit, nil
		}
	case AnalyticSphereCavity:
		p := m.sphereCavity
		if p.Local != nil {
			return p.Local.LengthUnit, nil
		}
		if p.Placed != nil {
			return p.Placed.Local.LengthUnit, nil
		}
	case AnalyticConeCavity:
		p := m.coneCavity
		if p.Local != nil {
			return p.Local.LengthUnit, nil
		}
		if p.Placed != nil {
			return p.Placed.Local.LengthUnit, nil
		}
	case AnalyticCylinderSphereCavity:
		p := m.cylinderSphereCavity
		if p.Local != nil {
			return p.Local.LengthUnit, nil
		}
		if p.Placed != nil {
			return p.Placed.Local.LengthUnit, nil
		}
	case AnalyticConeCylinderThrough:
		p := m.coneCylinderThrough
		if p.Local != nil {
			return p.Local.LengthUnit, nil
		}
		if p.Placed != nil {
			return p.Placed.Local.LengthUnit, nil
		}
	}
	return 0, geom.ErrInvalid
}
