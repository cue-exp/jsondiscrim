// Package jsondiscrim holds (experimental and hacky) support for unmarshalling
// discriminated unions from JSON in Go.
package jsondiscrim

import (
	"bytes"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// Structs returns an unmarshaler that unmarshals the given type T
// (which should be an interface type) by consulting the struct fields
// of each of the given choices to decide which concrete type to
// unmarshal into.
//
// All of choices should be different struct types (or pointers to
// struct types) that all contain a single common field of type [Const]
// with a different constant value for each choice. The value of that
// field is then inspected at unmarshal time to determine which actual
// type to unmarshal into.
func Structs[T any](choices ...T) *json.Unmarshalers {
	return StructsWithFallback(*new(T), choices...)
}

// StructsWithFallback is like [Structs] except that the concrete type
// of the first argument is used as a fallback choice for unmarshaling
// when none of the other choices apply.
func StructsWithFallback[T any](fallback T, choices ...T) *json.Unmarshalers {
	if t := reflect.TypeFor[T](); t.Kind() != reflect.Interface {
		panic(fmt.Errorf("type %v is not an interface type", t))
	}
	hasFallback := !isNil(fallback)
	if len(choices) == 0 && !hasFallback {
		panic("no choices provided to Structs")
	}
	var discrimField string
	var discrimByValue map[any]T
	if len(choices) > 0 {
		discrimField, discrimByValue = determineDiscriminator(choices)
	}
	if discrimField == "" {
		// No discriminator but we do have a fallback.
		// In this case, we don't have to buffer the value
		// and can just do the simple direct unmarshal.
		return json.UnmarshalFromFunc(func(d *jsontext.Decoder, src *T) error {
			zero := fallback
			reflect.ValueOf(&zero).Elem().Set(
				reflect.New(reflect.TypeOf(zero).Elem()),
			)
			if err := json.UnmarshalDecode(d, zero); err != nil {
				return err
			}
			*src = zero
			return nil
		})
	}
	return json.UnmarshalFromFunc(func(d *jsontext.Decoder, src *T) error {
		raw, err := d.ReadValue()
		if err != nil {
			return err
		}
		knownDiscrim := false
		var zero T
		if discrimField != "" {
			discrimValue, err := fieldValue(raw, discrimField)
			if err != nil && !hasFallback {
				return err
			}
			if err == nil {
				zero, knownDiscrim = discrimByValue[discrimValue]
				if !knownDiscrim &&  !hasFallback {
					return fmt.Errorf("unknown discriminator value %q (valid values are %v)", discrimValue, maps.Keys(discrimByValue))
				}
			}
		}
		if !knownDiscrim {
			zero = fallback
		}
		reflect.ValueOf(&zero).Elem().Set(
			reflect.New(reflect.TypeOf(zero).Elem()),
		)
		if err := json.Unmarshal(raw, zero, d.Options()); err != nil {
			return err
		}
		*src = zero
		return nil
	})
}

func determineDiscriminator[T any](choices []T) (discrimField string, discrimByValue map[any] T) {
	discrims := make(map[string]map[any]T)
	for i, choice := range choices {
		if isNil(choice) {
			panic(fmt.Errorf("argument %d is nil but should be concrete implementation of %v", i, reflect.TypeFor[T]()))
		}
		for fieldName, v := range constFields(reflect.TypeOf(choice)) {
			byValue := discrims[fieldName]
			if discrims[fieldName] == nil {
				byValue = make(map[any]T)
				discrims[fieldName] = byValue
			}
			byValue[v] = choice
		}
	}
	for fieldName, byValue := range discrims {
		if len(byValue) != len(choices) {
			continue
		}
		if discrimField != "" {
			panic(fmt.Errorf("ambiguous discriminator fields %q and %q", discrimField, fieldName))
		}
		discrimField = fieldName
		discrimByValue = byValue
	}
	if discrimField == "" {
		panic(fmt.Errorf("cannot determine discriminator from possibles %v", slices.Sorted(maps.Keys(discrims))))
	}
	return discrimField, discrimByValue
}

func constFields(t0 reflect.Type) map[string]any {
	t := t0
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Errorf("argument to Structs is %v not struct or pointer-to-struct", t0))
	}
	fields := make(map[string]any)
	for _, f := range reflect.VisibleFields(t) {
		if f.PkgPath != "" {
			continue
		}
		fv, ok := reflect.Zero(f.Type).Interface().(interface {
			constValue() any
		})
		if !ok {
			continue
		}
		name := jsonFieldName(f)
		if _, ok := fields[name]; ok {
			panic(fmt.Errorf("multiple fields with JSON name %q in %v", name, t0))
		}
		fields[name] = fv.constValue()
	}
	return fields
}

func jsonFieldName(f reflect.StructField) string {
	name := f.Name
	tag := f.Tag.Get("json")
	if tag != "" {
		jsonName, _, _ := strings.Cut(tag, ",")
		if jsonName != "" {
			name = jsonName
		}
	}
	return name
}

func lookupJSONField(t reflect.Type, jsonName string) reflect.Type {
	for _, f := range reflect.VisibleFields(t) {
		if f.PkgPath != "" {
			continue
		}
		if jsonFieldName(f) == jsonName {
			return f.Type
		}
	}
	return nil
}

func fieldValue(data []byte, fieldName string) (any, error) {
	d := jsontext.NewDecoder(bytes.NewBuffer(data))
	tok, err := d.ReadToken()
	if err != nil {
		return nil, err
	}
	if tok.Kind() != '{' {
		return nil, fmt.Errorf("expected object, got %v", tok.Kind())
	}
	for {
		tok, err := d.ReadToken()
		if err != nil {
			return nil, err
		}
		if tok.Kind() == '}' {
			return nil, fmt.Errorf("discriminator field %q not found", fieldName)
		}
		if tok.Kind() != '"' {
			return nil, fmt.Errorf("unexpected token %q", tok)
		}
		if tok.String() != fieldName {
			if err := d.SkipValue(); err != nil {
				return nil, err
			}
			continue
		}
		var v any
		if err := json.UnmarshalDecode(d, &v); err != nil {
			return nil, err
		}
		return v, nil
	}
}

func isNil[T any](x T) bool {
	return reflect.ValueOf(&x).Elem().IsNil()
}
