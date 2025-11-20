package jsondiscrim

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/go-json-experiment/json"
)

type constInfo[T any] struct {
	valueType reflect.Type
	value     T
}

func (c *constInfo[T]) getValueType() reflect.Type {
	return c.valueType
}

// Const represents a constant JSON-marshalable value as a type. T
// represents the type of the constant, and S is a struct that defines
// the actual constant.
//
// S must be a struct containing a single field. That field's tag must
// hold a "const" key with the  value of the constant.
//
// For example:
//
//	Const[string, struct{string `const:"foo bar"`}]
//
// represents the constant value "foo bar".
//
// A Const value always marshals to JSON as the constant's value, and
// when unmarshaling, requires the unmarshaled value to be equal to the
// constant's value.
type Const[T comparable, S any] struct{}

func (v Const[T, S]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Value())
}

func (v *Const[T, S]) UnmarshalJSON(data []byte) error {
	val := v.Value()
	var got T
	if err := json.Unmarshal(data, &got); err != nil {
		return err
	}

	if got != val {
		return fmt.Errorf("unexpected const value; got %#v but want %#v", got, val)
	}
	return nil
}

var constByType sync.Map // typeCmp[S]{}-> *constInfo

type typeCmp[T any] struct{}

// Value returns the constant value for v.
func (v Const[T, S]) Value() T {
	structType := typeCmp[S]{}
	// Ensure we only do the reflection work once.
	info0, ok := constByType.Load(structType)
	if !ok {
		constByType.LoadOrStore(structType, v.makeConstInfo())
		return v.Value()
	}
	info, ok := info0.(*constInfo[T])
	if !ok {
		info := info0.(interface{ getValueType() reflect.Type })
		panic(fmt.Errorf("struct field type %v does not agree with type parameter %v", info.getValueType(), reflect.TypeFor[T]()))
	}
	return info.value
}

func (v Const[T, S]) constValue() any {
	return v.Value()
}

func (Const[T, S]) makeConstInfo() *constInfo[T] {
	t := reflect.TypeFor[S]()
	if t.Kind() != reflect.Struct {
		panic(fmt.Errorf("const type argument is not struct"))
	}
	if t.NumField() != 1 {
		panic(fmt.Errorf("const type argument is not struct"))
	}
	if t.Field(0).Type != reflect.TypeFor[T]() {
		panic(fmt.Errorf("struct field type does not agree with type parameter"))
	}
	jsonVal, ok := t.Field(0).Tag.Lookup("const")
	if !ok {
		panic(fmt.Errorf("const type argument field has no const tag"))
	}

	var constVal T
	constValv := reflect.ValueOf(&constVal).Elem()
	if constValv.Kind() == reflect.String {
		constValv.SetString(jsonVal)
	} else {
		if err := json.Unmarshal([]byte(jsonVal), &constVal); err != nil {
			panic(fmt.Errorf("malformed const struct field tag %q", jsonVal))
		}
	}
	return &constInfo[T]{
		valueType: constValv.Type(),
		value:     constVal,
	}
}
