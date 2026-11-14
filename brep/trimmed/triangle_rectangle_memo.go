package trimmed

import "plainsolid/geom"

// triangleRectangleMemo is local to one grid call and retains at most one
// immutable restricted patch. It reuses only Trim/Bounds, never a triangle
// residual or its validation. The original patch pointer binds construction
// limits as well as geometry. Failed preparation is never cached.
type triangleRectangleMemo struct {
	source, restricted *geom.SurfacePatch
	domain             geom.ParameterRectangle
	box                geom.Box
}

func (m *triangleRectangleMemo) prepare(p *geom.SurfacePatch, d geom.ParameterRectangle) (*geom.SurfacePatch, geom.Box, error) {
	if m != nil && m.source == p && m.domain == d {
		return m.restricted, m.box, nil
	}
	q, e := p.Trim(d)
	if e != nil {
		return nil, geom.Box{}, e
	}
	b, e := q.Bounds()
	if e != nil {
		return nil, geom.Box{}, e
	}
	if m != nil {
		*m = triangleRectangleMemo{source: p, restricted: q, domain: d, box: b}
	}
	return q, b, nil
}
