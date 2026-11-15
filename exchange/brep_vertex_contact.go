package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"plainsolid/brep"
	"plainsolid/collision"
)

// VertexContactAssembly preserves separate original indexed planar solids and
// their declared point contacts. A reader returns it only after exact geometric
// verification. The component indices and their vertex IDs never get welded.
type VertexContactAssembly struct {
	Components []brep.Solid
	Contacts   []collision.AssemblyVertexContact
}

// VertexContactAssemblyJSONOptions limits the whole file, each component's
// native topology, and the exact assembly proof independently. Zero MaxBytes
// sets a 16 MiB whole-file limit. Component limits follow BRepLimits; its byte
// limit applies to the component's compact canonical encoding.
type VertexContactAssemblyJSONOptions struct {
	MaxBytes  int64
	Component BRepLimits
	Proof     collision.VertexContactAssemblyOptions
}

type vertexContactAssemblyDocument struct {
	Version    int                               `json:"version"`
	Kind       string                            `json:"kind"`
	Components []brepDocument                    `json:"components"`
	Contacts   []collision.AssemblyVertexContact `json:"contacts"`
}

// WriteVertexContactAssemblyJSON writes original component graphs and a
// canonical vertex-contact ledger. No display triangles or cached geometric
// proofs are written. Validation completes before bytes reach the writer.
func WriteVertexContactAssemblyJSON(ctx context.Context, w io.Writer, a VertexContactAssembly, o VertexContactAssemblyJSONOptions) error {
	if ctx == nil || w == nil {
		return collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	o, err := vertexContactJSONOptions(o)
	if err != nil {
		return err
	}
	if len(a.Components) > o.Proof.MaxSolids || len(a.Contacts) > o.Proof.MaxDeclaredContacts {
		return collision.ErrLimit
	}
	doc := vertexContactAssemblyDocument{Version: 1, Kind: "planar-vertex-contact-assembly", Components: make([]brepDocument, len(a.Components))}
	for i, c := range a.Components {
		if err := ctx.Err(); err != nil {
			return err
		}
		doc.Components[i] = brepDocument{1, "planar-brep", c.Vertices(), c.Edges(), c.Faces()}
		if err := checkVertexContactComponent(doc.Components[i], o); err != nil {
			return fmt.Errorf("assembly component %d: %w", i, err)
		}
	}
	report, err := collision.ValidateVertexContactAssembly(ctx, a.Components, a.Contacts, o.Proof)
	if err != nil {
		return err
	}
	doc.Contacts = append(make([]collision.AssemblyVertexContact, 0, len(report.Contacts)), report.Contacts...)
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if int64(len(raw)) > o.MaxBytes {
		return collision.ErrLimit
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}

// ReadVertexContactAssemblyJSON verifies every original component and the
// complete exact contact graph. A failure returns neither partial material nor
// partial diagnostics. The ordinary planar BRep reader retains its strict
// single-model contract.
func ReadVertexContactAssemblyJSON(ctx context.Context, r io.Reader, o VertexContactAssemblyJSONOptions) (VertexContactAssembly, collision.VertexContactAssemblyReport, error) {
	var zero VertexContactAssembly
	var noReport collision.VertexContactAssemblyReport
	if ctx == nil || r == nil {
		return zero, noReport, collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, noReport, err
	}
	o, err := vertexContactJSONOptions(o)
	if err != nil {
		return zero, noReport, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, o.MaxBytes+1))
	if err != nil {
		return zero, noReport, err
	}
	if int64(len(raw)) > o.MaxBytes {
		return zero, noReport, collision.ErrLimit
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var d vertexContactAssemblyDocument
	if err := dec.Decode(&d); err != nil {
		return zero, noReport, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF || d.Version != 1 || d.Kind != "planar-vertex-contact-assembly" || d.Components == nil || d.Contacts == nil {
		return zero, noReport, collision.ErrInvalid
	}
	if len(d.Components) > o.Proof.MaxSolids || len(d.Contacts) > o.Proof.MaxDeclaredContacts {
		return zero, noReport, collision.ErrLimit
	}
	a := VertexContactAssembly{Components: make([]brep.Solid, len(d.Components))}
	for i, doc := range d.Components {
		if err := ctx.Err(); err != nil {
			return zero, noReport, err
		}
		if doc.Version != 1 || doc.Kind != "planar-brep" {
			return zero, noReport, fmt.Errorf("%w: assembly component %d schema", collision.ErrInvalid, i)
		}
		if err := checkVertexContactComponent(doc, o); err != nil {
			return zero, noReport, fmt.Errorf("assembly component %d: %w", i, err)
		}
		a.Components[i], err = brep.NewSolid(doc.Vertices, doc.Edges, doc.Faces)
		if err != nil {
			return zero, noReport, fmt.Errorf("assembly component %d: %w", i, err)
		}
	}
	report, err := collision.ValidateVertexContactAssembly(ctx, a.Components, d.Contacts, o.Proof)
	if err != nil {
		return zero, noReport, err
	}
	a.Contacts = append([]collision.AssemblyVertexContact(nil), report.Contacts...)
	return a, report, nil
}

func checkVertexContactComponent(doc brepDocument, o VertexContactAssemblyJSONOptions) error {
	if err := checkBRepDocument(doc, o.Component); err != nil {
		return err
	}
	if o.Component.MaxBytes < o.MaxBytes {
		raw, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		if int64(len(raw)) > o.Component.MaxBytes {
			return collision.ErrLimit
		}
	}
	return nil
}

func vertexContactJSONOptions(o VertexContactAssemblyJSONOptions) (VertexContactAssemblyJSONOptions, error) {
	if o.MaxBytes < 0 || o.MaxBytes > 1<<30 || o.Proof.MaxSolids < 0 || o.Proof.MaxDeclaredContacts < 0 {
		return o, collision.ErrInvalid
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = 16 << 20
	}
	if o.Proof.MaxSolids == 0 {
		o.Proof.MaxSolids = 256
	}
	if o.Proof.MaxDeclaredContacts == 0 {
		o.Proof.MaxDeclaredContacts = 1024
	}
	var err error
	o.Component, err = o.Component.normalized()
	return o, err
}
