package cylindercells

import (
	"bytes"
	"encoding/json"
	"io"
)

const MaxNativeBytes = 2 << 20

type envelope struct {
	Format  string
	Version int
	Asset   Asset
}

// WriteNative writes version-one neutral geometry with optional bounded source
// evidence. Geometry and retained metadata binding are proved before output.
func WriteNative(w io.Writer, a Asset) error {
	if w == nil {
		return ErrAsset
	}
	if _, e := a.Validate(); e != nil {
		return e
	}
	b, e := json.Marshal(envelope{"plainsolid-cylinder-cells", 1, a})
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > MaxNativeBytes {
		return ErrAsset
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadNative rejects duplicate/unknown fields, nulls, unsupported versions,
// trailing data and stale geometry/source metadata. Failure is all-or-none.
func ReadNative(r io.Reader) (Asset, error) {
	if r == nil {
		return Asset{}, ErrAsset
	}
	b, e := io.ReadAll(io.LimitReader(r, MaxNativeBytes+1))
	if e != nil {
		return Asset{}, e
	}
	if len(b) > MaxNativeBytes {
		return Asset{}, ErrAsset
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if e = jsonUnique(d, 0); e != nil {
		return Asset{}, e
	}
	if _, e = d.Token(); e != io.EOF {
		return Asset{}, ErrAsset
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var v envelope
	if e = d.Decode(&v); e != nil {
		return Asset{}, e
	}
	if v.Format != "plainsolid-cylinder-cells" || v.Version != 1 {
		return Asset{}, ErrAsset
	}
	if _, e = v.Asset.Validate(); e != nil {
		return Asset{}, e
	}
	return v.Asset, nil
}
func jsonUnique(d *json.Decoder, depth int) error {
	if depth > 8 {
		return ErrAsset
	}
	t, e := d.Token()
	if e != nil {
		return e
	}
	if t == nil {
		return ErrAsset
	}
	q, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	if q != '{' && q != '[' {
		return ErrAsset
	}
	seen := map[string]bool{}
	for d.More() {
		if q == '{' {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			if !ok || !nativeKey(s) || seen[s] {
				return ErrAsset
			}
			seen[s] = true
		}
		if e = jsonUnique(d, depth+1); e != nil {
			return e
		}
	}
	_, e = d.Token()
	return e
}

// Require canonical field spelling rather than encoding/json's Unicode
// case-fold aliases, which could otherwise bypass duplicate-key checks.
func nativeKey(s string) bool {
	switch s {
	case "Format", "Version", "Asset", "Dialect", "Name", "Unit", "MetresExponent", "Origin", "Reference", "Axis", "X", "Y", "Z", "Radius", "Height", "SourceSTEP":
		return true
	}
	return false
}
