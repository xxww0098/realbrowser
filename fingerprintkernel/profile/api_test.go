package profile_test

import (
	"encoding/json"
	"testing"

	"github.com/xxww0098/realbrowser/fingerprintkernel/profile"
)

func TestMergeFingerprintJSON(t *testing.T) {
	const original = `{"lang":"en","timezone":"UTC","later":9007199254740993,"permissions":{"later":true}}`
	spec := profile.FingerprintSpec{Lang: "fr-FR"}
	raw, err := profile.MergeFingerprintJSON(original, spec)
	var saved struct {
		Lang, Timezone string
		Later          int64
		Permissions    struct{ Later bool }
	}
	if err != nil || json.Unmarshal([]byte(raw), &saved) != nil || saved.Lang != "fr-FR" || saved.Timezone != "" || saved.Later != 9007199254740993 || !saved.Permissions.Later {
		t.Fatalf("merged: %s, %v", raw, err)
	}
	if fresh, err := profile.MergeFingerprintJSON("", spec); err != nil || !json.Valid([]byte(fresh)) {
		t.Fatalf("new document: %q, %v", fresh, err)
	}
	if raw, err := profile.MergeFingerprintJSON("{", spec); err == nil || raw != "" {
		t.Fatalf("malformed source: %q, %v", raw, err)
	}
	spec.HardwareConcurrency = -1
	if raw, err := profile.MergeFingerprintJSON(original, spec); err == nil || raw != "" {
		t.Fatalf("invalid spec: %q, %v", raw, err)
	}
}
