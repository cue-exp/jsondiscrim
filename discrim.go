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
// All the choices should be concrete values that contain a single
// common field of type [ConstValue] with a different value for each
// choice.
func Structs[T any](choices ...T) *json.Unmarshalers {
	if len(choices) == 0 {
		panic("no choices provided to Structs")
	}
	discrims := make(map[string]map[any]T)
	for _, choice := range choices {
		for fieldName, v := range constFields(reflect.TypeOf(choice)) {
			byValue := discrims[fieldName]
			if discrims[fieldName] == nil {
				byValue = make(map[any]T)
				discrims[fieldName] = byValue
			}
			byValue[v] = choice
		}
	}
	var discrimField string
	var discrimByValue map[any]T
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
	return json.UnmarshalFromFunc(func(d *jsontext.Decoder, src *T) error {
		raw, err := d.ReadValue()
		if err != nil {
			return err
		}
		discrimValue, err := fieldValue(raw, discrimField)
		if err != nil {
			return err
		}
		zero, ok := discrimByValue[discrimValue]
		if !ok {
			return fmt.Errorf("unknown discriminator value %q (valid values are %v)", discrimValue, maps.Keys(discrimByValue))
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
		name := f.Name
		tag := f.Tag.Get("json")
		if tag != "" {
			jsonName, _, _ := strings.Cut(tag, ",")
			if jsonName != "" {
				name = jsonName
			}
		}
		if _, ok := fields[name]; ok {
			panic(fmt.Errorf("multiple fields with JSON name %q in %v", name, t0))
		}
		fields[name] = fv.constValue()
	}
	return fields
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
