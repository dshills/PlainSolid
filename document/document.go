// Package document provides assemblies with mesh, planar and parametric assets,
// snapshot transactions and bounded undo/redo.
package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"sync"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange"
	"plainsolid/geom"
	"plainsolid/mesh"
)

var ErrConflict = errors.New("document: revision conflict")
var ErrHistory = errors.New("document: no history")

// Node instances an optional mesh or B-rep asset. Parent is empty for a root node.
// Transform maps local coordinates into parent coordinates. Use geom.Identity
// explicitly: the zero affine transform is singular and rejected.
type Node struct {
	Name      string
	Parent    string
	Asset     string
	Transform geom.Affine
}

// State holds stable caller-selected IDs. LengthUnit is meters per coordinate
// unit; positive finite values are required. Asset meshes may be open surfaces.
type State struct {
	LengthUnit float64
	Assets     map[string]mesh.Mesh
	Nodes      map[string]Node
	// Labels store document, node and asset metadata by stable ID.
	Labels     map[string]Label            `json:",omitempty"`
	Planar     map[string]brep.Solid       `json:"-"`
	Parametric map[string]parametric.Solid `json:"-"`
	Trimmed    map[string]trimmed.Solid    `json:"-"`
	Features   map[string]Feature          `json:",omitempty"`
}

func clone(s State) State {
	r := State{LengthUnit: s.LengthUnit, Assets: make(map[string]mesh.Mesh, len(s.Assets)), Nodes: make(map[string]Node, len(s.Nodes))}
	for id, m := range s.Assets {
		r.Assets[id] = mesh.Mesh{Vertices: append([]geom.Vec3(nil), m.Vertices...), Triangles: append([][3]int(nil), m.Triangles...)}
	}
	for id, n := range s.Nodes {
		r.Nodes[id] = n
	}
	if s.Labels != nil {
		r.Labels = make(map[string]Label, len(s.Labels))
		for id, l := range s.Labels {
			r.Labels[id] = l.clone()
		}
	}
	if s.Planar != nil {
		r.Planar = make(map[string]brep.Solid, len(s.Planar))
		for id, solid := range s.Planar {
			r.Planar[id] = solid
		}
	}
	if s.Parametric != nil {
		r.Parametric = make(map[string]parametric.Solid, len(s.Parametric))
		for id, solid := range s.Parametric {
			r.Parametric[id] = solid
		}
	}
	if s.Trimmed != nil {
		r.Trimmed = make(map[string]trimmed.Solid, len(s.Trimmed))
		for id, v := range s.Trimmed {
			r.Trimmed[id] = v
		}
	}
	if s.Features != nil {
		r.Features = make(map[string]Feature, len(s.Features))
		for id, f := range s.Features {
			r.Features[id] = f.clone()
		}
	}
	return r
}
func (s State) Validate() error {
	if len(s.Features) > 0 {
		m, e := s.EvaluateFeatures(context.Background())
		if e != nil {
			return e
		}
		return m.Validate()
	}
	if math.IsNaN(s.LengthUnit) || math.IsInf(s.LengthUnit, 0) || s.LengthUnit <= 0 {
		return errors.New("document: invalid length unit")
	}
	for id, m := range s.Assets {
		if id == "" {
			return errors.New("document: empty asset ID")
		}
		if e := m.Validate(); e != nil {
			return fmt.Errorf("document: asset %q: %w", id, e)
		}
	}
	for id := range s.Planar {
		if _, conflict := s.Assets[id]; id == "" || conflict {
			return fmt.Errorf("document: invalid or repeated asset ID %q", id)
		}
	}
	for id := range s.Parametric {
		_, meshConflict := s.Assets[id]
		_, planarConflict := s.Planar[id]
		if id == "" || meshConflict || planarConflict {
			return fmt.Errorf("document: invalid or repeated asset ID %q", id)
		}
	}
	for id := range s.Trimmed {
		_, meshConflict := s.Assets[id]
		_, planarConflict := s.Planar[id]
		_, patchConflict := s.Parametric[id]
		if id == "" || meshConflict || planarConflict || patchConflict {
			return fmt.Errorf("document: invalid or repeated asset ID %q", id)
		}
	}

	for id, n := range s.Nodes {
		if id == "" {
			return errors.New("document: empty node ID")
		}
		if n.Parent != "" {
			if _, ok := s.Nodes[n.Parent]; !ok {
				return fmt.Errorf("document: node %q missing parent", id)
			}
		}
		if n.Asset != "" {
			if _, a := s.Assets[n.Asset]; !a {
				_, b := s.Planar[n.Asset]
				_, c := s.Parametric[n.Asset]
				_, d := s.Trimmed[n.Asset]
				if !b && !c && !d {
					return fmt.Errorf("document: node %q missing asset", id)
				}
			}
		}
		if _, e := n.Transform.Inverse(); e != nil {
			return fmt.Errorf("document: node %q invalid transform: %w", id, e)
		}
	}
	if e := s.validateLabels(); e != nil {
		return e
	}
	_, e := s.worldTransforms()
	return e
}

// worldTransforms walks each node at most twice and uses one consistent
// parent-to-child multiplication order for validation and export.
func (s State) worldTransforms() (map[string]geom.Affine, error) {
	done := make(map[string]geom.Affine, len(s.Nodes))
	marks := make(map[string]bool, len(s.Nodes))
	path := make([]string, 0)
	for id := range s.Nodes {
		if _, ok := done[id]; ok {
			continue
		}
		path = path[:0]
		cur := id
		for cur != "" {
			if _, ok := done[cur]; ok {
				break
			}
			if marks[cur] {
				return nil, errors.New("document: cyclic assembly")
			}
			n, ok := s.Nodes[cur]
			if !ok {
				return nil, errors.New("document: missing parent")
			}
			marks[cur] = true
			path = append(path, cur)
			cur = n.Parent
		}
		world := geom.Identity()
		if cur != "" {
			world = done[cur]
		}
		for i := len(path) - 1; i >= 0; i-- {
			node := path[i]
			world = world.Mul(s.Nodes[node].Transform)
			if _, e := world.Inverse(); e != nil {
				return nil, fmt.Errorf("document: accumulated transform for %q: %w", node, e)
			}
			done[node] = world
		}
	}
	return done, nil
}

// WorldTransform returns the accumulated local-to-world transform.
func (s State) WorldTransform(id string) (geom.Affine, error) {
	if _, ok := s.Nodes[id]; !ok {
		return geom.Affine{}, errors.New("document: unknown node")
	}
	var chain []geom.Affine
	seen := map[string]bool{}
	for id != "" {
		if seen[id] {
			return geom.Affine{}, errors.New("document: cyclic assembly")
		}
		seen[id] = true
		n, ok := s.Nodes[id]
		if !ok {
			return geom.Affine{}, errors.New("document: missing parent")
		}
		if _, e := n.Transform.Inverse(); e != nil {
			return geom.Affine{}, e
		}
		chain = append(chain, n.Transform)
		id = n.Parent
	}
	r := geom.Identity()
	for i := len(chain) - 1; i >= 0; i-- {
		r = r.Mul(chain[i])
		if _, e := r.Inverse(); e != nil {
			return geom.Affine{}, fmt.Errorf("document: accumulated transform: %w", e)
		}
	}
	return r, nil
}

// Flatten expands every asset instance into a combined world-coordinate mesh,
// preserving orientation under reflections. It does not Boolean-union instances.
// Nodes are visited in ID order for deterministic output. Units are unchanged.
func (s State) Flatten() (mesh.Mesh, error) {
	return s.FlattenWithLimits(3000000, 1000000)
}

// FlattenWithLimits bounds the expanded vertex and triangle counts, including
// unused vertices. Limits must be positive. Flatten uses 3M/1M defaults.
func (s State) FlattenWithLimits(maxVertices, maxTriangles int) (mesh.Mesh, error) {
	return s.FlattenGeometry(context.Background(), maxVertices, maxTriangles, parametric.TessellationOptions{})
}

// FlattenGeometry emits a world-space mesh using the provided deflection
// in asset coordinates. Instances reuse each asset's local triangulation.
// Curved assets require an explicit positive deflection.
func (s State) FlattenGeometry(ctx context.Context, maxVertices, maxTriangles int, options parametric.TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil {
		return mesh.Mesh{}, errors.New("document: nil context")
	}
	if maxVertices <= 0 || maxTriangles <= 0 {
		return mesh.Mesh{}, errors.New("document: invalid expansion limits")
	}

	if len(s.Features) > 0 {
		m, e := s.EvaluateFeatures(ctx)
		if e != nil {
			return mesh.Mesh{}, e
		}
		s = m
	}
	if e := s.Validate(); e != nil {
		return mesh.Mesh{}, e
	}
	vertices, triangles := 0, 0
	cache := map[string]mesh.Mesh{}

	for _, n := range s.Nodes {
		if n.Asset != "" {
			if e := ctx.Err(); e != nil {
				return mesh.Mesh{}, e
			}
			m, ok := cache[n.Asset]
			if !ok {
				var e error
				m, e = s.localMesh(ctx, n.Asset, options, maxTriangles-triangles)
				if e != nil {
					return mesh.Mesh{}, e
				}
				cache[n.Asset] = m
			}
			if len(m.Vertices) > maxVertices-vertices || len(m.Triangles) > maxTriangles-triangles {
				return mesh.Mesh{}, errors.New("document: expanded mesh exceeds limits")
			}
			vertices += len(m.Vertices)
			triangles += len(m.Triangles)
		}
	}
	ids := make([]string, 0, len(s.Nodes))
	for id := range s.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	transforms, e := s.worldTransforms()
	if e != nil {
		return mesh.Mesh{}, e
	}
	r := mesh.Mesh{Vertices: make([]geom.Vec3, 0, vertices), Triangles: make([][3]int, 0, triangles)}
	for _, id := range ids {
		if e := ctx.Err(); e != nil {
			return mesh.Mesh{}, e
		}
		n := s.Nodes[id]
		if n.Asset == "" {
			continue
		}
		tr := transforms[id]
		m, e := cache[n.Asset].Transformed(tr)
		if e != nil {
			return mesh.Mesh{}, e
		}
		offset := len(r.Vertices)
		r.Vertices = append(r.Vertices, m.Vertices...)
		for _, f := range m.Triangles {
			r.Triangles = append(r.Triangles, [3]int{f[0] + offset, f[1] + offset, f[2] + offset})
		}
	}
	return r, nil
}

// Document is safe for concurrent snapshot, commit, undo and redo operations.
// HistoryLimit bounds snapshot count, not bytes; choose it for your model sizes.
type Document struct {
	mu           sync.RWMutex
	state        State
	revision     uint64
	undo, redo   []State
	historyLimit int
}

func New(s State, historyLimit int) (*Document, error) {
	if historyLimit < 0 {
		return nil, errors.New("document: negative history limit")
	}
	if e := s.Validate(); e != nil {
		return nil, e
	}
	return &Document{state: clone(s), historyLimit: historyLimit}, nil
}
func (d *Document) Snapshot() (State, uint64) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return clone(d.state), d.revision
}

// Transaction owns its mutable State. A transaction must not be shared between
// goroutines; independent transactions on a Document are supported.
type Transaction struct {
	State    State
	doc      *Document
	revision uint64
	closed   bool
}

func (d *Document) Begin() *Transaction {
	s, r := d.Snapshot()
	return &Transaction{State: s, doc: d, revision: r}
}
func (t *Transaction) Rollback() { t.closed = true; t.State = State{} }
func (t *Transaction) Commit() error {
	if t == nil || t.doc == nil || t.closed {
		return errors.New("document: closed transaction")
	}
	if e := t.State.Validate(); e != nil {
		return e
	}
	d := t.doc
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.revision != t.revision {
		return ErrConflict
	}
	if d.historyLimit > 0 {
		d.undo = append(d.undo, d.state)
		if len(d.undo) > d.historyLimit {
			d.undo = append([]State(nil), d.undo[len(d.undo)-d.historyLimit:]...)
		}
	}
	d.state = clone(t.State)
	d.redo = nil
	d.revision++
	t.closed = true
	return nil
}
func (d *Document) Undo() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.undo) == 0 {
		return ErrHistory
	}
	d.redo = append(d.redo, d.state)
	d.state = d.undo[len(d.undo)-1]
	d.undo = d.undo[:len(d.undo)-1]
	d.revision++
	return nil
}
func (d *Document) Redo() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.redo) == 0 {
		return ErrHistory
	}
	d.undo = append(d.undo, d.state)
	d.state = d.redo[len(d.redo)-1]
	d.redo = d.redo[:len(d.redo)-1]
	d.revision++
	return nil
}

type snapshotFile struct {
	Version    int                        `json:"version"`
	State      State                      `json:"state"`
	Planar     map[string]json.RawMessage `json:"planar,omitempty"`
	Parametric map[string]json.RawMessage `json:"parametric,omitempty"`
	Trimmed    map[string]json.RawMessage `json:"trimmed,omitempty"`
}

// Write persists a snapshot. Version 2 adds exact assets; version 1 is
// retained for mesh-only snapshots.
func (d *Document) Write(w io.Writer) error {
	if w == nil {
		return errors.New("document: nil writer")
	}
	s, _ := d.Snapshot()
	file := snapshotFile{Version: 1, State: s}
	if len(s.Planar) > 0 || len(s.Parametric) > 0 || len(s.Features) > 0 || len(s.Trimmed) > 0 {
		file.Version = 2
		file.Planar = map[string]json.RawMessage{}
		file.Parametric = map[string]json.RawMessage{}
		for id, v := range s.Planar {
			var buf bytes.Buffer
			if err := exchange.WriteBRepJSON(&buf, v); err != nil {
				return fmt.Errorf("document: planar asset %q: %w", id, err)
			}
			file.Planar[id] = buf.Bytes()
		}
		for id, v := range s.Parametric {
			var buf bytes.Buffer
			if err := parametric.WriteJSON(&buf, v); err != nil {
				return fmt.Errorf("document: parametric asset %q: %w", id, err)
			}
			file.Parametric[id] = buf.Bytes()
		}
		if len(s.Trimmed) > 0 {
			file.Trimmed = make(map[string]json.RawMessage, len(s.Trimmed))
			for id, v := range s.Trimmed {
				var b bytes.Buffer
				if err := trimmed.WriteJSON(&b, v); err != nil {
					return fmt.Errorf("document: trimmed asset %q: %w", id, err)
				}
				file.Trimmed[id] = b.Bytes()
			}
		}
		file.State.Planar = nil
	}
	b, e := json.Marshal(file)
	if e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}

// Read accepts a bounded version 1 or 2 snapshot. maxBytes=0 selects 64 MiB.
func Read(r io.Reader, maxBytes int64, historyLimit int) (*Document, error) {
	if r == nil {
		return nil, errors.New("document: nil reader")
	}
	if maxBytes < 0 || maxBytes == math.MaxInt64 {
		return nil, errors.New("document: invalid byte limit")
	}
	if maxBytes == 0 {
		maxBytes = 64 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > maxBytes {
		return nil, errors.New("document: byte limit exceeded")
	}
	var v snapshotFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&v); e != nil {
		return nil, e
	}
	var extra any
	if e = dec.Decode(&extra); e != io.EOF {
		return nil, errors.New("document: trailing data")
	}
	if v.Version != 1 && v.Version != 2 || v.Version == 1 && (len(v.Planar) > 0 || len(v.Parametric) > 0 || len(v.State.Features) > 0 || len(v.Trimmed) > 0) || v.Version == 2 && len(v.Planar) == 0 && len(v.Parametric) == 0 && len(v.State.Features) == 0 && len(v.Trimmed) == 0 {
		return nil, errors.New("document: unsupported snapshot schema")
	}
	if len(v.Planar) > 0 {
		v.State.Planar = map[string]brep.Solid{}
	}
	if len(v.Parametric) > 0 {
		v.State.Parametric = map[string]parametric.Solid{}
	}
	for id, b := range v.Planar {
		s, err := exchange.ReadBRepJSON(bytes.NewReader(b), exchange.BRepLimits{MaxBytes: maxBytes})
		if err != nil {
			return nil, fmt.Errorf("document: planar asset %q: %w", id, err)
		}
		v.State.Planar[id] = s
	}
	for id, b := range v.Parametric {
		s, err := parametric.ReadJSON(bytes.NewReader(b), parametric.JSONLimits{MaxBytes: min(maxBytes, 32<<20)})
		if err != nil {
			return nil, fmt.Errorf("document: parametric asset %q: %w", id, err)
		}
		v.State.Parametric[id] = s
	}
	if len(v.Trimmed) > 0 {
		v.State.Trimmed = make(map[string]trimmed.Solid, len(v.Trimmed))
	}
	for id, b := range v.Trimmed {
		s, err := trimmed.ReadJSON(bytes.NewReader(b), trimmed.JSONLimits{MaxBytes: min(maxBytes, 32<<20)})
		if err != nil {
			return nil, fmt.Errorf("document: trimmed asset %q: %w", id, err)
		}
		v.State.Trimmed[id] = s
	}

	return New(v.State, historyLimit)
}
