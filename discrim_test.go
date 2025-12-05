package jsondiscrim

import (
	stdjson "encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/go-quicktest/qt"
	"github.com/google/go-cmp/cmp"
)

type stringConst[S any] = Const[string, S]

type Valuer[T any] interface {
	Value() T
}

func TestConstValue(t *testing.T) {
	tests := []struct {
		name     string
		constVal any
		want     any
	}{
		{"string foo", stringConst[struct {
			string `const:"foo"`
		}]{}, "foo"},
		{"string bar", stringConst[struct {
			string `const:"bar"`
		}]{}, "bar"},
		{"string baz", stringConst[struct {
			string `const:"baz"`
		}]{}, "baz"},
		{"int 42", Const[int, struct {
			int `const:"42"`
		}]{}, 42},
		{"bool true", Const[bool, struct {
			bool `const:"true"`
		}]{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got any
			switch v := tt.constVal.(type) {
			case Valuer[string]:
				got = v.Value()
			case Valuer[int]:
				got = v.Value()
			case Valuer[bool]:
				got = v.Value()
			default:
				t.Fatalf("unexpected type %T", tt.constVal)
			}
			qt.Assert(t, qt.Equals(got, tt.want))
		})
	}
}

func TestConstMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		constVal any
		want     string
	}{
		{"string foo", stringConst[struct {
			string `const:"foo"`
		}]{}, `"foo"`},
		{"string bar", stringConst[struct {
			string `const:"bar"`
		}]{}, `"bar"`},
		{"int 42", Const[int, struct {
			int `const:"42"`
		}]{}, `42`},
		{"bool true", Const[bool, struct {
			bool `const:"true"`
		}]{}, `true`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := stdjson.Marshal(tt.constVal)
			qt.Assert(t, qt.IsNil(err))
			qt.Assert(t, qt.Equals(string(data), tt.want))
		})
	}
}

func TestConstUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		target  any
		wantErr bool
		errMsg  string
	}{
		{
			name: "string foo success",
			json: `"foo"`,
			target: new(stringConst[struct {
				string `const:"foo"`
			}]),
		},
		{
			name: "string foo wrong value",
			json: `"bar"`,
			target: new(stringConst[struct {
				string `const:"foo"`
			}]),
			wantErr: true,
			errMsg:  `unexpected const value; got "bar" but want "foo"`,
		},
		{
			name: "int 42 success",
			json: `42`,
			target: new(Const[int, struct {
				int `const:"42"`
			}]),
		},
		{
			name: "int 42 wrong value",
			json: `99`,
			target: new(Const[int, struct {
				int `const:"42"`
			}]),
			wantErr: true,
			errMsg:  "unexpected const value; got 99 but want 42",
		},
		{
			name: "bool true success",
			json: `true`,
			target: new(Const[bool, struct {
				bool `const:"true"`
			}]),
		},
		{
			name: "bool true wrong value",
			json: `false`,
			target: new(Const[bool, struct {
				bool `const:"true"`
			}]),
			wantErr: true,
			errMsg:  "unexpected const value; got false but want true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := stdjson.Unmarshal([]byte(tt.json), tt.target)
			if tt.wantErr {
				qt.Assert(t, qt.ErrorMatches(err, tt.errMsg))
			} else {
				qt.Assert(t, qt.IsNil(err))
			}
		})
	}
}

type BaseAnimal[S any] struct {
	Type stringConst[S] `json:"type"`
}

// Test types for Structs discriminator functionality
type Animal interface {
	isAnimal()
}

type Dog struct {
	BaseAnimal[struct {
		string `const:"dog"`
	}]
	Bark string
}

func (Dog) isAnimal() {}

type Cat struct {
	BaseAnimal[struct {
		string `const:"cat"`
	}]
	Meow string
}

func (Cat) isAnimal() {}

type Bird struct {
	BaseAnimal[struct {
		string `const:"bird"`
	}]
	Sing string
}

func (Bird) isAnimal() {}

type OtherAnimal struct {
	Type        string         `json:"type"`
	OtherFields jsontext.Value `json:",unknown"`
}

func (OtherAnimal) isAnimal() {}

func TestStructsBasic(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    Animal
		wantErr string
	}{
		{
			name: "dog",
			json: `{"type":"dog","Bark":"woof"}`,
			want: &Dog{Bark: "woof"},
		},
		{
			name: "cat",
			json: `{"type":"cat","Meow":"meow"}`,
			want: &Cat{Meow: "meow"},
		},
		{
			name: "bird",
			json: `{"type":"bird","Sing":"tweet"}`,
			want: &Bird{Sing: "tweet"},
		},
		{
			name: "fallback",
			json: `{"type":"dragon", "A": true}`,
			want: &OtherAnimal{Type: "dragon", OtherFields: jsontext.Value(`{"A":true}`)},
		},
		{
			name: "missing discriminator",
			json: `{"Data":"test"}`,
			want: &OtherAnimal{OtherFields: jsontext.Value(`{"Data":"test"}`)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Animal
			err := json.Unmarshal([]byte(tt.json), &got, json.WithUnmarshalers(StructsWithFallback[Animal](
				(*OtherAnimal)(nil),
				(*Dog)(nil),
				(*Cat)(nil),
				(*Bird)(nil),
			)))
			if tt.wantErr != "" {
				qt.Assert(t, qt.ErrorMatches(err, tt.wantErr))
			} else {
				qt.Assert(t, qt.IsNil(err))
				qt.Assert(t, qt.DeepEquals(got, tt.want))
			}
		})
	}
}

func TestDiscriminator(t *testing.T) {
	field, byValue, err := Discriminator[Animal](
		(*Dog)(nil),
		(*Cat)(nil),
		Bird{},
	)
	qt.Assert(t, qt.IsNil(err))
	qt.Assert(t, qt.Equals(field, "type"))
	qt.Assert(t, qt.CmpEquals(byValue, map[any]reflect.Type{
		"dog":  reflect.TypeFor[*Dog](),
		"cat":  reflect.TypeFor[*Cat](),
		"bird": reflect.TypeFor[Bird](),
	}, cmp.Comparer(cmpWithEqual[reflect.Type])))
}

func TestStructsWithFallbackOnly(t *testing.T) {
	// This exercises the slightly different path in the logic
	// when there's a fallback with no choices.
	type S struct {
		A Animal
	}
	var got S
	err := json.Unmarshal(
		[]byte(`{"A": {"type": "a", "foo": true}}`),
		&got,
		json.WithUnmarshalers(StructsWithFallback[Animal]((*OtherAnimal)(nil))))
	qt.Assert(t, qt.IsNil(err))
	qt.Assert(t, qt.DeepEquals(got, S{
		A: &OtherAnimal{
			Type:        "a",
			OtherFields: jsontext.Value(`{"foo":true}`),
		},
	}))
}

type Vehicle interface {
	isVehicle()
}

type Car struct {
	Kind stringConst[struct {
		string `const:"car"`
	}]
	Brand string
}

func (Car) isVehicle() {}

type Bike struct {
	Kind stringConst[struct {
		string `const:"bike"`
	}]
	Gears int
}

func (Bike) isVehicle() {}

func TestStructsWithDifferentFieldName(t *testing.T) {
	tests := []struct {
		name string
		json string
		want Vehicle
	}{
		{
			name: "car",
			json: `{"Kind":"car","Brand":"Toyota"}`,
			want: &Car{Brand: "Toyota"},
		},
		{
			name: "bike",
			json: `{"Kind":"bike","Gears":21}`,
			want: &Bike{Gears: 21},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Vehicle
			err := json.Unmarshal([]byte(tt.json), &got, json.WithUnmarshalers(Structs[Vehicle](
				(*Car)(nil),
				(*Bike)(nil),
			)))
			qt.Assert(t, qt.IsNil(err))
			qt.Assert(t, qt.DeepEquals(got, tt.want))
		})
	}
}

type Item interface {
	isItem()
}

type Book struct {
	Type stringConst[struct {
		string `const:"book"`
	}] `json:"type"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

func (Book) isItem() {}

type Movie struct {
	Type stringConst[struct {
		string `const:"movie"`
	}] `json:"type"`
	Title    string `json:"title"`
	Director string `json:"director"`
}

func (Movie) isItem() {}

func TestStructsWithJSONTags(t *testing.T) {
	tests := []struct {
		name string
		json string
		want Item
	}{
		{
			name: "book",
			json: `{"type":"book","title":"1984","author":"George Orwell"}`,
			want: &Book{Title: "1984", Author: "George Orwell"},
		},
		{
			name: "movie",
			json: `{"type":"movie","title":"Inception","director":"Christopher Nolan"}`,
			want: &Movie{Title: "Inception", Director: "Christopher Nolan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Item
			err := json.Unmarshal([]byte(tt.json), &got, json.WithUnmarshalers(Structs[Item](
				(*Book)(nil),
				(*Movie)(nil),
			)))
			qt.Assert(t, qt.IsNil(err))
			qt.Assert(t, qt.DeepEquals(got, tt.want))
		})
	}
}

func TestStructsPanics(t *testing.T) {
	t.Run("no choices", func(t *testing.T) {
		qt.Assert(t, qt.PanicMatches(func() {
			Structs[Animal]()
		}, "no choices provided to Structs"))
	})

	type NoDiscrim1 struct {
		Field1 string
	}
	type NoDiscrim2 struct {
		Field2 int
	}

	t.Run("no discriminator field", func(t *testing.T) {
		qt.Assert(t, qt.PanicMatches(func() {
			Structs[any](&NoDiscrim1{}, &NoDiscrim2{})
		}, "cannot determine discriminator.*"))
	})

	type Ambig1 struct {
		Field1 stringConst[struct {
			string `const:"foo"`
		}]
		Field2 stringConst[struct {
			string `const:"bar"`
		}]
	}
	type Ambig2 struct {
		Field1 stringConst[struct {
			string `const:"bar"`
		}]
		Field2 stringConst[struct {
			string `const:"foo"`
		}]
	}

	t.Run("ambiguous discriminator", func(t *testing.T) {
		qt.Assert(t, qt.PanicMatches(func() {
			Structs[any](&Ambig1{}, &Ambig2{})
		}, "ambiguous discriminator fields.*"))
	})

	type NotStruct int

	t.Run("non-struct choice", func(t *testing.T) {
		qt.Assert(t, qt.PanicMatches(func() {
			Structs[any](NotStruct(0))
		}, ".*not struct.*"))
	})
}

func TestFieldValue(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		field   string
		want    any
		wantErr string
	}{
		{
			name:  "string field",
			json:  `{"name":"John","age":30}`,
			field: "name",
			want:  "John",
		},
		{
			name:  "int field",
			json:  `{"name":"John","age":30}`,
			field: "age",
			want:  float64(30), // JSON numbers unmarshal as float64
		},
		{
			name:  "nested object",
			json:  `{"user":{"name":"John"},"age":30}`,
			field: "age",
			want:  float64(30),
		},
		{
			name:  "first field",
			json:  `{"first":"value","second":"other"}`,
			field: "first",
			want:  "value",
		},
		{
			name:    "field not found",
			json:    `{"name":"John","age":30}`,
			field:   "missing",
			wantErr: `discriminator field "missing" not found`,
		},
		{
			name:    "not an object",
			json:    `["array"]`,
			field:   "name",
			wantErr: ".*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fieldValue([]byte(tt.json), tt.field)
			if tt.wantErr != "" {
				qt.Assert(t, qt.ErrorMatches(err, tt.wantErr))
			} else {
				qt.Assert(t, qt.IsNil(err))
				qt.Assert(t, qt.DeepEquals(got, tt.want))
			}
		})
	}
}

func TestConstFields(t *testing.T) {
	type TestStruct struct {
		Discrim stringConst[struct {
			string `const:"foo"`
		}]
		Data string
	}

	fields := constFields(reflect.TypeOf(TestStruct{}))
	qt.Assert(t, qt.Equals(len(fields), 1))
	qt.Assert(t, qt.Equals(fields["Discrim"], "foo"))

	// Test with pointer type
	fields = constFields(reflect.TypeOf((*TestStruct)(nil)))
	qt.Assert(t, qt.Equals(len(fields), 1))

	// Test with JSON tags
	type TestStructWithTag struct {
		Discrim stringConst[struct {
			string `const:"bar"`
		}] `json:"type"`
		Data string
	}

	fields = constFields(reflect.TypeOf(TestStructWithTag{}))
	qt.Assert(t, qt.Equals(fields["type"], "bar"))
	_, exists := fields["Discrim"]
	qt.Assert(t, qt.IsFalse(exists))
}

func TestConstFieldsPanics(t *testing.T) {
	t.Run("non-struct type", func(t *testing.T) {
		qt.Assert(t, qt.PanicMatches(func() {
			constFields(reflect.TypeOf(42))
		}, ".*not struct.*"))
	})

	type DuplicateJSON struct {
		Field1 stringConst[struct {
			string `const:"foo"`
		}] `json:"same"`
		Field2 stringConst[struct {
			string `const:"bar"`
		}] `json:"same"`
	}

	t.Run("duplicate JSON names", func(t *testing.T) {
		qt.Assert(t, qt.PanicMatches(func() {
			constFields(reflect.TypeOf(DuplicateJSON{}))
		}, "multiple fields with JSON name.*"))
	})
}

// Test round-trip marshaling and unmarshaling
func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input Animal
	}{
		{"dog", &Dog{Bark: "woof woof"}},
		{"cat", &Cat{Meow: "meow meow"}},
		{"bird", &Bird{Sing: "chirp chirp"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the input
			data, err := json.Marshal(tt.input)
			qt.Assert(t, qt.IsNil(err))

			// Unmarshal back
			var got Animal
			err = json.Unmarshal(data, &got, json.WithUnmarshalers(Structs[Animal](
				(*Dog)(nil),
				(*Cat)(nil),
				(*Bird)(nil),
			)))
			qt.Assert(t, qt.IsNil(err))

			// Compare
			qt.Assert(t, qt.DeepEquals(got, tt.input))
		})
	}
}

// Test with unexported fields (should be ignored)
type WithUnexported struct {
	Type stringConst[struct {
		string `const:"foo"`
	}]
	unexported string
	Eexported  string
}

func TestConstFieldsIgnoresUnexported(t *testing.T) {
	fields := constFields(reflect.TypeOf(WithUnexported{}))
	qt.Assert(t, qt.Equals(len(fields), 1))
}

// Test decoder at different positions in JSON
func TestStructsWithFieldOrder(t *testing.T) {
	tests := []struct {
		name string
		json string
		want Animal
	}{
		{
			name: "discriminator first",
			json: `{"type":"dog","Bark":"first"}`,
			want: &Dog{Bark: "first"},
		},
		{
			name: "discriminator last",
			json: `{"Bark":"last","type":"dog"}`,
			want: &Dog{Bark: "last"},
		},
		{
			name: "discriminator middle",
			json: `{"Bark":"middle","type":"cat","Meow":"purr"}`,
			want: &Cat{Meow: "purr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Animal
			err := json.Unmarshal([]byte(tt.json), &got, json.WithUnmarshalers(Structs[Animal](
				(*Dog)(nil),
				(*Cat)(nil),
			)))
			qt.Assert(t, qt.IsNil(err))
			qt.Assert(t, qt.DeepEquals(got, tt.want))
		})
	}
}

// Test error messages for invalid discriminator values
func TestStructsErrorMessages(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantErrMsg string
	}{
		{
			name:       "unknown discriminator value",
			json:       `{"type":"invalid","Data":"test"}`,
			wantErrMsg: ".*unknown discriminator value.*",
		},
		{
			name:       "missing discriminator",
			json:       `{"Data":"test"}`,
			wantErrMsg: `.*discriminator field "type" not found.*`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Animal
			err := json.Unmarshal([]byte(tt.json), &got, json.WithUnmarshalers(Structs[Animal](
				(*Dog)(nil),
				(*Cat)(nil),
			)))
			qt.Assert(t, qt.ErrorMatches(err, tt.wantErrMsg))
		})
	}
}

// Test that Value() is consistent across multiple calls
func TestConstValueConsistency(t *testing.T) {
	cv := stringConst[struct {
		string `const:"foo"`
	}]{}
	val1 := cv.Value()
	val2 := cv.Value()
	val3 := cv.Value()

	qt.Assert(t, qt.Equals(val1, val2))
	qt.Assert(t, qt.Equals(val2, val3))
}

// Test integration with jsontext.Decoder using UnmarshalDecode
func TestStructsWithDecoder(t *testing.T) {
	jsonData := `{"type":"dog","Bark":"decoder test"}`
	dec := jsontext.NewDecoder(strings.NewReader(jsonData))

	var got Animal
	err := json.UnmarshalDecode(dec, &got, json.WithUnmarshalers(Structs[Animal](
		Dog{},
		Cat{},
	)))
	qt.Assert(t, qt.IsNil(err))

	want := Animal(Dog{Bark: "decoder test"})
	qt.Assert(t, qt.DeepEquals(got, want))
}

func cmpWithEqual[T comparable](x, y T) bool {
	return x == y
}
