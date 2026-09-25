package service

import (
	"sort"
	"testing"

	"ntdkhiem/ppbudget-go/internal/repository"
)

// The spec table and the repository's column registry must name the same
// fields: a column with no spec would be a question no screen knows how to
// ask, and a spec with no column one no answer could be saved to.
func TestFieldSpecsCoverTheRegistry(t *testing.T) {
	registry := map[string]bool{}
	for _, key := range repository.ProfileFieldKeys() {
		registry[key] = true
		if _, ok := profileFields[key]; !ok {
			t.Errorf("profile field %q has no spec", key)
		}
	}
	for key := range profileFields {
		if !registry[key] {
			t.Errorf("spec %q names no profile field", key)
		}
	}
}

// Every spec is askable: a label, a kind the client renders, and choices
// wherever the kind is a choice.
func TestFieldSpecsAreAskable(t *testing.T) {
	kinds := map[string]bool{
		FieldKindMoney: true, FieldKindPercent: true, FieldKindInteger: true, FieldKindBoolean: true,
		FieldKindDate: true, FieldKindText: true, FieldKindChoice: true,
	}
	for key, spec := range profileFields {
		if spec.Label == "" {
			t.Errorf("%s has no label", key)
		}
		if !kinds[spec.Kind] {
			t.Errorf("%s has unknown kind %q", key, spec.Kind)
		}
		if (spec.Kind == FieldKindChoice) != (len(spec.Choices) > 0) {
			t.Errorf("%s: choices belong exactly to choice fields", key)
		}
	}
	for key := range pseudoFieldLabels {
		if _, clash := profileFields[key]; clash {
			t.Errorf("%s is both a field and a pseudo-field", key)
		}
	}
}

// Served in a stable order, and never listing a correction as a question.
func TestProfileFieldsAreServedInOrder(t *testing.T) {
	fields := (&Service{}).ProfileFields()
	if !sort.SliceIsSorted(fields, func(i, j int) bool { return fields[i].Key < fields[j].Key }) {
		t.Error("fields should come back sorted by key")
	}
	for _, f := range fields {
		if f.Correction != isOverrideField(f.Key) {
			t.Errorf("%s: correction flag and isOverrideField disagree", f.Key)
		}
	}
}
