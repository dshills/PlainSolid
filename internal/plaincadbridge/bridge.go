// Package plaincadbridge provides a bounded JSON bridge for the initial planar
// PlainCAD integration. Handles own immutable B-reps; meshes are derived output.
// Unsupported curves and operations must be rejected by the caller, never
// polygonized and labeled as exact geometry.
package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/csg/draft"
	"plainsolid/csg/poly"
	"plainsolid/exchange"
	"plainsolid/geom"
	"plainsolid/mesh"
)

const MaxRequestBytes = 1 << 20
const MaxHandles = 256
const MaxProfileVertices = 128

type vector [3]float64

// encoding/json accepts null into a float64 as its zero value. Coordinates,
// transform entries and wall thicknesses must instead be actual JSON numbers.
type number float64

func (n *number) UnmarshalJSON(b []byte) error {
	var value *float64
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	if value == nil {
		return errors.New("expected a JSON number, not null")
	}
	*n = number(*value)
	return nil
}

func (v *vector) UnmarshalJSON(b []byte) error {
	var a []number
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	if len(a) != 3 {
		return errors.New("coordinates require exactly three numbers")
	}
	for i, value := range a {
		v[i] = float64(value)
	}
	return nil
}
func (v vector) vec() geom.Vec3      { return geom.Vec3{X: v[0], Y: v[1], Z: v[2]} }
func coordinates(v geom.Vec3) vector { return vector{v.X, v.Y, v.Z} }

type edgeReference struct {
	Start *vector `json:"start"`
	End   *vector `json:"end"`
}

type request struct {
	MaxSegments       int                  `json:"maxSegments,omitempty"`
	EdgeUses          *int                 `json:"edgeUses,omitempty"`
	Op                string               `json:"op"`
	Handle            string               `json:"handle,omitempty"`
	A                 string               `json:"a,omitempty"`
	B                 string               `json:"b,omitempty"`
	Tools             []string             `json:"tools,omitempty"`
	Min               *vector              `json:"min,omitempty"`
	Max               *vector              `json:"max,omitempty"`
	Outer             []vector             `json:"outer,omitempty"`
	Holes             [][]vector           `json:"holes,omitempty"`
	Offset            *vector              `json:"offset,omitempty"`
	Matrix            [][]number           `json:"matrix,omitempty"`
	OpenFace          *int                 `json:"openFace,omitempty"`
	Wall              number               `json:"wall,omitempty"`
	Center            *vector              `json:"center,omitempty"`
	Direction         *vector              `json:"direction,omitempty"`
	Origin            *vector              `json:"origin,omitempty"`
	Normal            *vector              `json:"normal,omitempty"`
	RimCenters        []vector             `json:"rimCenters,omitempty"`
	Edges             []edgeReference      `json:"edges,omitempty"`
	EdgeStart         *vector              `json:"edgeStart,omitempty"`
	EdgeEnd           *vector              `json:"edgeEnd,omitempty"`
	Distance          number               `json:"distance,omitempty"`
	Radius            number               `json:"radius,omitempty"`
	Source            string               `json:"source,omitempty"`
	Handles           []string             `json:"handles,omitempty"`
	Lead              *leadRequest         `json:"lead,omitempty"`
	Nut               bool                 `json:"nut,omitempty"`
	Thread            *threadRequest       `json:"thread,omitempty"`
	Profile           *mixedProfileRequest `json:"profile,omitempty"`
	Angle             number               `json:"angle,omitempty"`
	Height            number               `json:"height,omitempty"`
	LinearDeflection  optionalNumber       `json:"linearDeflection,omitempty"`
	AngularDeflection optionalNumber       `json:"angularDeflection,omitempty"`
}

type Assertions struct {
	Valid       bool    `json:"valid"`
	Volume      float64 `json:"volume"`
	SurfaceArea float64 `json:"surfaceArea"`
	SolidCount  int     `json:"solidCount"`
}
type Bounds struct {
	Min vector `json:"min"`
	Max vector `json:"max"`
}
type Shape struct {
	Handle             string     `json:"handle"`
	GeometryAssertions Assertions `json:"geometryAssertions"`
	Bounds             Bounds     `json:"bounds"`
}
type stored struct {
	solid    brep.Solid
	cylinder *cylinderBody
	bore     *boreBody
	treated  *edgeBody
	analytic *analyticBody
	result   Shape
}

func (s stored) planar() bool {
	return s.cylinder == nil && s.bore == nil && s.treated == nil && s.analytic == nil
}

type failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type response struct {
	OK     bool     `json:"ok"`
	Result any      `json:"result,omitempty"`
	Error  *failure `json:"error,omitempty"`
}

// Bridge is an isolated handle registry. Calls are serialized, and all state
// disappears when its owning worker exits. A browser call is synchronous;
// cancellation requires terminating that worker, not a queued cancel message.
type Bridge struct {
	mu     sync.Mutex
	next   uint64
	shapes map[string]stored
}

func New() *Bridge { return &Bridge{shapes: make(map[string]stored)} }

func encode(r response) string {
	b, err := json.Marshal(r)
	if err != nil {
		return `{"ok":false,"error":{"code":"INTERNAL","message":"response contains invalid geometry"}}`
	}
	return string(b)
}

// Call accepts one strict JSON request and returns a JSON response. It never
// exposes native pointers, and failed operations do not publish handles.
func (b *Bridge) Call(input string) (output string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer func() {
		if recover() != nil {
			output = encode(response{Error: &failure{"INTERNAL", "kernel operation failed"}})
		}
	}()
	bad := func(code string, err error) string { return encode(response{Error: &failure{code, err.Error()}}) }
	if len(input) > MaxRequestBytes {
		return bad("LIMIT", errors.New("request exceeds 1 MiB"))
	}
	var q request
	d := json.NewDecoder(bytes.NewBufferString(input))
	d.DisallowUnknownFields()
	if err := d.Decode(&q); err != nil {
		return bad("INVALID", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return bad("INVALID", errors.New("expected one JSON request"))
	}
	r, err := b.execute(q)
	if err != nil {
		code := "INVALID"
		var e *failureError
		if errors.As(err, &e) {
			code = e.code
		} else if errors.Is(err, poly.ErrLimit) || errors.Is(err, collision.ErrLimit) || errors.Is(err, parametric.ErrBudget) {
			code = "LIMIT"
		}
		return bad(code, err)
	}
	return encode(response{OK: true, Result: r})
}

type failureError struct{ code, message string }

func (e *failureError) Error() string { return e.message }

func (b *Bridge) get(h string) (stored, error) {
	s, ok := b.shapes[h]
	if !ok {
		return stored{}, &failureError{"HANDLE", "unknown or disposed shape handle"}
	}
	return s, nil
}
func loop(vs []vector) []geom.Vec3 {
	out := make([]geom.Vec3, len(vs))
	for i, v := range vs {
		out[i] = v.vec()
	}
	return out
}
func (b *Bridge) execute(q request) (any, error) {
	ctx := context.Background()
	switch q.Op {
	case "capabilities":
		return map[string]any{"schema": 1, "kernel": "plainsolid", "domain": "planar_and_circular_brep", "operations": []string{"box", "prism", "prismToFace", "circleToFace", "circle", "mixedExtrude", "revolvePolygon", "revolveCircle", "edgeAvailable", "fillet", "filletCylinderRims", "chamfer", "capChamfer", "capFillet", "cut", "cutMany", "overlapVolume", "fuse", "joinAll", "common", "shell", "planarFace", "pocketFloor", "holeRim", "drawingGeometry", "displayEdges", "thread", "leadPart", "assertLeadPair", "importStep", "exportStep", "clone", "transform", "inspect", "mesh", "stl", "dispose"}, "maxHandles": MaxHandles, "maxProfileVertices": MaxProfileVertices, "cancellation": "terminate-worker"}, nil
	case "leadPart", "assertLeadPair":
		if q.Lead == nil {
			return nil, errors.New("lead screw operation requires dimensions")
		}
		if q.Op == "leadPart" {
			return b.createLeadPart(*q.Lead, q.Nut)
		}
		a, err := b.get(q.A)
		if err != nil {
			return nil, err
		}
		c, err := b.get(q.B)
		if err != nil {
			return nil, err
		}
		if err := b.checkLeadPair(a, c, *q.Lead); err != nil {
			return nil, err
		}
		return map[string]bool{"compatible": true}, nil
	case "thread":
		if q.Thread == nil {
			return nil, errors.New("thread requires dimensions and handedness")
		}
		s, err := b.get(q.Handle)
		if err != nil {
			return nil, err
		}
		return b.threadBody(s, *q.Thread)
	case "importStep":
		return b.importSTEP(q.Source)
	case "exportStep":
		return b.exportSTEP(q.Handles)
	case "pocketFloor", "holeRim":
		s, err := b.get(q.Handle)
		if err != nil {
			return nil, err
		}
		if q.Op == "holeRim" {
			return s.holeRim(q)
		}
		if err := s.pocketFloor(q); err != nil {
			return nil, err
		}
		return map[string]bool{"available": true}, nil
	case "drawingGeometry":
		s, err := b.get(q.Handle)
		if err != nil {
			return nil, err
		}
		return s.drawingGeometry()
	case "prismToFace", "circleToFace":
		return b.toFace(q)
	case "mixedExtrude":
		if q.Profile == nil {
			return nil, errors.New("mixedExtrude requires a profile")
		}
		return b.mixedExtrude(*q.Profile)
	case "revolvePolygon", "revolveCircle":
		return b.revolve(q)
	case "edgeAvailable", "fillet", "filletCylinderRims", "chamfer", "capChamfer", "capFillet":
		s, err := b.get(q.Handle)
		if err != nil {
			return nil, err
		}
		if q.Op == "capChamfer" || q.Op == "capFillet" {
			return b.treatCapPerimeter(s, q)
		}
		if q.Op == "filletCylinderRims" {
			return b.filletCylinderRims(s, q)
		}
		if q.Op == "edgeAvailable" {
			if q.EdgeStart == nil || q.EdgeEnd == nil {
				return nil, errors.New("edge endpoints required")
			}
			return map[string]bool{"available": s.edgeAvailable(q.EdgeStart.vec(), q.EdgeEnd.vec())}, nil
		}
		return b.treatEdge(s, q)
	case "circle":
		if q.Origin == nil || q.Normal == nil {
			return nil, errors.New("circle requires origin and normal")
		}
		return b.circle(q.Origin.vec(), q.Normal.vec(), float64(q.Radius), float64(q.Height))
	case "box":
		if q.Min == nil || q.Max == nil {
			return nil, errors.New("box requires min and max")
		}
		s, err := brep.Box(q.Min.vec(), q.Max.vec())
		if err != nil {
			return nil, err
		}
		return b.put(s)
	case "prism":
		if q.Offset == nil {
			return nil, errors.New("prism requires offset")
		}
		n := len(q.Outer)
		for _, h := range q.Holes {
			n += len(h)
		}
		if n > MaxProfileVertices {
			return nil, &failureError{"LIMIT", "profile exceeds 128 vertices"}
		}
		holes := make([][]geom.Vec3, len(q.Holes))
		for i, h := range q.Holes {
			holes[i] = loop(h)
		}
		s, err := brep.PrismWithHoles(loop(q.Outer), holes, q.Offset.vec())
		if err != nil {
			return nil, err
		}
		return b.put(s)
	case "overlapVolume":
		return b.overlapVolume(q.A, q.B)
	case "cutMany":
		return b.cutMany(q.A, q.Tools)
	case "joinAll":
		return b.joinAll(q.Handles, q.B)
	case "cut", "fuse", "common":
		a, err := b.get(q.A)
		if err != nil {
			return nil, err
		}
		c, err := b.get(q.B)
		if err != nil {
			return nil, err
		}
		if q.Op == "cut" {
			if c.planar() {
				if result, handled, err := b.rectangularPocket(a, c); handled {
					return result, err
				}
			}
			if a.analytic != nil && a.analytic.pockets != nil && c.cylinder != nil {
				p, err := a.analytic.pockets.throughBore(c.cylinder)
				if err != nil {
					return nil, err
				}
				return b.putPocketStock(p)
			}
			if a.analytic != nil && a.analytic.shaft != nil && c.planar() {
				return b.cutShaftSocket(*a.analytic.shaft, c.solid)
			}
			if sector, ok := coaxialThrough(a, c.cylinder); ok {
				sector.inner = c.cylinder.radius
				return b.putRadialSector(sector)
			}
		}
		if a.analytic != nil || c.analytic != nil {
			return nil, &failureError{"UNSUPPORTED", "Boolean operation is outside the supported analytic stock domain"}
		}
		if result, handled, err := b.cornerCylinderBoolean(a, c, q.Op); handled {
			return result, err
		}
		if q.Op == "fuse" && a.bore != nil && c.cylinder != nil {
			if result, handled, err := b.fuseCircularBoss(a.bore, c.cylinder); handled {
				return result, err
			}
		}
		if q.Op == "fuse" && a.bore != nil && c.planar() {
			if result, handled, err := b.fuseBoredStock(a.bore, c.solid); handled {
				return result, err
			}
		}
		if a.treated != nil || c.treated != nil {
			return b.treatedBoolean(a, c, q.Op)
		}
		if a.cylinder != nil || c.cylinder != nil || a.bore != nil || c.bore != nil {
			return b.curvedBoolean(a, c, q.Op)
		}
		if q.Op == "fuse" {
			if box, ok := joinedRectangularStock(a.solid, c.solid); ok {
				return b.put(box)
			}
		}
		op := poly.Difference
		if q.Op == "fuse" {
			op = poly.Union
		}
		if q.Op == "common" {
			op = poly.Intersection
		}
		s, err := poly.Combine(ctx, a.solid, c.solid, op, poly.Options{})
		if err != nil {
			return nil, err
		}
		return b.put(s)
	case "displayEdges", "clone", "inspect", "mesh", "stl", "dispose", "transform", "shell", "planarFace":
		s, err := b.get(q.Handle)
		if err != nil {
			return nil, err
		}
		switch q.Op {
		case "displayEdges":
			return nativeDisplayEdges(s, q.MaxSegments, q.EdgeUses)
		case "planarFace":
			if q.Origin == nil || q.Normal == nil {
				return nil, errors.New("planarFace requires origin and normal")
			}
			if s.treated != nil {
				return s.treated.planarFace(q.Origin.vec(), q.Normal.vec())
			}
			if s.bore != nil {
				return s.bore.planarFace(q.Origin.vec(), q.Normal.vec())
			}
			if s.cylinder != nil {
				return s.cylinder.planarFace(q.Origin.vec(), q.Normal.vec())
			}
			if s.analytic != nil {
				return s.analytic.planarFace(q.Origin.vec(), q.Normal.vec())
			}
			return matchingPlanarFace(s.solid, q.Origin.vec(), q.Normal.vec())
		case "shell":
			if !s.planar() {
				return nil, &failureError{"UNSUPPORTED", "shell requires planar stock"}
			}
			if q.OpenFace == nil || *q.OpenFace < 0 || *q.OpenFace >= len(s.solid.Faces()) || q.Wall <= 0 {
				return nil, errors.New("shell requires a valid opening face and positive wall thickness")
			}
			thickness := make([]float64, len(s.solid.Faces()))
			for i := range thickness {
				if i != *q.OpenFace {
					thickness[i] = float64(q.Wall)
				}
			}
			hollow, err := draft.Hollow(ctx, s.solid, thickness, draft.HollowOptions{OpenFace: *q.OpenFace})
			if err != nil {
				return nil, err
			}
			return b.put(hollow.Solid)
		case "inspect":
			return s.result, nil
		case "clone":
			return b.publish(s)
		case "dispose":
			delete(b.shapes, q.Handle)
			return map[string]bool{"disposed": true}, nil
		case "transform":
			if len(q.Matrix) != 3 {
				return nil, errors.New("matrix requires three rows of four numbers")
			}
			var m geom.Affine
			for i, row := range q.Matrix {
				if len(row) != 4 {
					return nil, errors.New("matrix requires three rows of four numbers")
				}
				for j, value := range row {
					m[i][j] = float64(value)
				}
			}
			if s.treated != nil {
				body, err := s.treated.transformed(m)
				if err != nil {
					return nil, err
				}
				return b.putEdge(body)
			}
			if s.bore != nil {
				body, err := s.bore.transformed(m)
				if err != nil {
					return nil, err
				}
				return b.putBore(body)
			}
			if s.cylinder != nil {
				c, err := s.cylinder.transformed(m)
				if err != nil {
					return nil, err
				}
				return b.putCylinder(c)
			}
			if s.analytic != nil {
				body, err := s.analytic.transformed(m)
				if err != nil {
					return nil, err
				}
				return b.putAnalyticBody(body)
			}
			t, err := s.solid.Transformed(m)
			if err != nil {
				return nil, err
			}
			return b.put(t)
		case "mesh":
			return renderMesh(s, q)
		case "stl":
			var out bytes.Buffer
			m, err := s.tessellate(q)
			if err != nil {
				return nil, err
			}
			if err := exchange.WriteSTL(&out, m); err != nil {
				return nil, err
			}
			return map[string]string{"base64": base64.StdEncoding.EncodeToString(out.Bytes())}, nil
		}
	}
	return nil, &failureError{"UNSUPPORTED", fmt.Sprintf("operation %q is outside the planar bridge", q.Op)}
}

func (b *Bridge) put(s brep.Solid) (Shape, error) {
	if len(b.shapes) >= MaxHandles {
		return Shape{}, &failureError{"LIMIT", "dispose shapes before allocating more than 256 handles"}
	}
	r, err := provePlanar(s)
	if err != nil {
		return Shape{}, err
	}
	return b.publish(stored{solid: s, result: r})
}

func provePlanar(s brep.Solid) (Shape, error) {
	// Validate geometric embedding and shell nesting, rather than equating a
	// watertight display mesh with a valid material solid. Even-depth shells are
	// separate material components; inward cavity shells do not add solidCount.
	l, err := collision.NewPointLocator(context.Background(), s, collision.LocationOptions{})
	if err != nil {
		return Shape{}, err
	}
	r := Shape{GeometryAssertions: Assertions{Valid: true}}
	for _, shell := range l.Shells() {
		if shell.Depth%2 == 0 {
			r.GeometryAssertions.SolidCount++
		}
	}
	if len(s.Faces()) > 0 {
		p, err := s.MassProperties()
		if err != nil {
			return Shape{}, err
		}
		if p.Volume <= 0 {
			return Shape{}, errors.New("material must have positive volume")
		}
		r.GeometryAssertions.Volume = p.Volume
		r.GeometryAssertions.SurfaceArea = p.SurfaceArea
		bounds, err := s.Bounds()
		if err != nil {
			return Shape{}, err
		}
		r.Bounds = Bounds{coordinates(bounds.Min), coordinates(bounds.Max)}
	}
	return r, nil
}

type Mesh struct {
	Positions          []float64  `json:"positions"`
	Normals            []float64  `json:"normals"`
	Indices            []int      `json:"indices"`
	Bounds             Bounds     `json:"bounds"`
	GeometryAssertions Assertions `json:"geometryAssertions"`
}

type PlanarFace struct {
	Face   int     `json:"face"`
	Origin vector  `json:"origin"`
	Normal vector  `json:"normal"`
	Area   float64 `json:"area"`
}

// matchingPlanarFace follows PlainCAD's native reference contract: exactly one
// finite face on the authored support plane, with either orientation. This is
// not a point-in-face test and does not recover a split or deleted feature role.
// Tolerances match OpenCascadeKernel.validatePlanarFace
// (millimetres): parallel unit normals within 1e-9 in absolute dot product,
// support-plane distance at most 1e-7, and area greater than (1e-7)^2.
func matchingPlanarFace(s brep.Solid, origin, normal geom.Vec3) (PlanarFace, error) {
	n, err := normal.Unit()
	if err != nil || !origin.IsFinite() {
		return PlanarFace{}, errors.New("planarFace requires a finite origin and nonzero normal")
	}
	face := -1
	var plane geom.Plane
	for i := range s.Faces() {
		p, err := s.FacePlane(i)
		if err != nil {
			return PlanarFace{}, err
		}
		delta := p.Origin().Sub(origin)
		if !delta.IsFinite() {
			return PlanarFace{}, errors.New("planarFace support-plane comparison overflows")
		}
		if math.Abs(math.Abs(p.Normal().Dot(n))-1) > 1e-9 || math.Abs(delta.Dot(p.Normal())) > 1e-7 {
			continue
		}
		if face != -1 {
			return PlanarFace{}, errors.New("planar face reference is ambiguous after splitting")
		}
		face, plane = i, p
	}
	if face == -1 {
		return PlanarFace{}, errors.New("planar face reference no longer exists")
	}
	m := s.Tessellate()
	owners := s.TriangleFaces()
	area := 0.0
	for i, triangle := range m.Triangles {
		if owners[i] == face {
			a, b, c := m.Vertices[triangle[0]], m.Vertices[triangle[1]], m.Vertices[triangle[2]]
			area += b.Sub(a).Cross(c.Sub(a)).Length() / 2
		}
	}
	if math.IsNaN(area) || math.IsInf(area, 0) || area <= 1e-14 {
		return PlanarFace{}, errors.New("planar face reference has no finite measurable area")
	}
	return PlanarFace{Face: face, Origin: coordinates(plane.Origin()), Normal: coordinates(plane.Normal()), Area: area}, nil
}

func renderMesh(s stored, q request) (Mesh, error) {
	m, err := s.tessellate(q)
	if err != nil {
		return Mesh{}, err
	}
	return renderTriangles(s, m)
}
func renderTriangles(s stored, m mesh.Mesh) (Mesh, error) {
	r := Mesh{Positions: make([]float64, 0, len(m.Triangles)*9), Normals: make([]float64, 0, len(m.Triangles)*9), Indices: make([]int, 0, len(m.Triangles)*3), Bounds: s.result.Bounds, GeometryAssertions: s.result.GeometryAssertions}
	// Duplicate per triangle so sharp planar edges retain flat face normals.
	for _, t := range m.Triangles {
		n, err := m.Vertices[t[1]].Sub(m.Vertices[t[0]]).Cross(m.Vertices[t[2]].Sub(m.Vertices[t[0]])).Unit()
		if err != nil {
			return Mesh{}, err
		}
		for _, id := range t {
			v := m.Vertices[id]
			r.Indices = append(r.Indices, len(r.Indices))
			r.Positions = append(r.Positions, v.X, v.Y, v.Z)
			r.Normals = append(r.Normals, n.X, n.Y, n.Z)
		}
	}
	return r, nil
}
