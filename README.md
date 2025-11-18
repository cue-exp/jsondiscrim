# jsondiscrim

A Go package for unmarshaling JSON with discriminated unions (tagged unions) using type-safe constant values.

This package leverages the new experimental [encoding/json/v2](https://pkg.go.dev/encoding/json/v2) API to provide compile-time type safety when working with polymorphic JSON data. Currently to avoid the requirement to set GOEXPERIMENT, this depends on https://github.com/go-json-experiment/json rather than `encoding/json/v2` itself.

## When is this useful?

Use this package when you need to unmarshal JSON objects where a discriminator field determines the concrete type:

- **API responses** with a `type` or `kind` field that varies the structure
- **Event systems** where different events have different payloads
- **Message queues** with polymorphic message types
- **Configuration files** with variant sections
- Any JSON that uses tagged unions or discriminated unions

## Example

```go
import (
	"github.com/cue-exp/jsondiscrim"
	"github.com/go-json-experiment/json"
)

// Define an interface for your message types
type Message interface {
    Format() string
}

// Define concrete types with a Const discriminator field
type TextMessage struct {
    Type jsondiscrim.Const[string, struct{ string `const:"text"` }] `json:"type"`
    Text string                                                       `json:"text"`
}

type ImageMessage struct {
    Type jsondiscrim.Const[string, struct{ string `const:"image"` }] `json:"type"`
    URL  string                                                        `json:"url"`
    Alt  string                                                        `json:"alt"`
}

// Create an unmarshaler for the interface
unmarshalers := jsondiscrim.Structs[Message](
    (*TextMessage)(nil),
    (*ImageMessage)(nil),
)

// Unmarshal JSON that will be dispatched to the correct type
conversationJSON := `{
    "messages": [
        {"type":"text","text":"Hello!"},
        {"type":"image","url":"pic.jpg","alt":"A picture"}
    ]
}`

var conv Conversation
err := json.Unmarshal([]byte(conversationJSON), &conv,
    json.WithUnmarshalers(unmarshalers))
```

## The Const-Struct Idiom

The package uses the following pattern to encode constant values as types:

```go
Const[string, struct{ string `const:"foo"` }]
```

This reads as: "a constant of type `string` with the value `"foo"`".

### Breaking it down:

1. **`Const[T, S]`** - A generic type with two parameters:
   - `T` - the type of the constant (string, int, bool, etc.)
   - `S` - a struct that encodes the actual constant value

2. **`struct{ string `const:"foo"` }`** - An anonymous struct with:
   - One field of type `T` (here, `string`)
   - A struct tag `const:"foo"` containing the constant's value

3. **Type aliases for convenience**:
   ```go
   type MessageTypeText = Const[string, struct{ string `const:"text"` }]
   type MessageTypeImage = Const[string, struct{ string `const:"image"` }]
   ```

### Why this pattern?

This idiom provides:
- **Compile-time type safety** - each constant value is a distinct type
- **Low runtime overhead** - values are computed once via reflection
- **Automatic marshaling** - `Const` fields always marshal to their constant value
- **Validation on unmarshal** - unmarshaling fails if the value doesn't match

## How it works

1. **Define your types** - Each concrete type has a `Const` field with a unique value
2. **Call `Structs`** - Pass pointers to zero values of each concrete type
3. **Automatic detection** - `Structs` examines the types to find the discriminator field
4. **Type-safe unmarshaling** - JSON is dispatched to the correct concrete type based on the discriminator

The `Structs` function:
- Finds the common field across all types that has different `Const` values
- Creates an unmarshaler that reads the discriminator field from JSON
- Dispatches to the appropriate concrete type based on the value

## Migrating to encoding/json/v2

This package currently uses the experimental `github.com/go-json-experiment/json` package. When Go's `encoding/json/v2` moves out of experimental mode and into the standard library, this package will be updated to use the stdlib version.

## License

See [LICENSE](LICENSE) file.
