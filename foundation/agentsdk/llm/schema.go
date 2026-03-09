package llm

// Schema describes the structure of data for tool parameters or structured output.
type Schema struct {
	Type        SchemaType         `json:"type"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`  // For arrays
	Enum        []string           `json:"enum,omitempty"`   // For enums
	Format      string             `json:"format,omitempty"` // date-time, email, etc.
	Default     any                `json:"default,omitempty"`
	Nullable    bool               `json:"nullable,omitempty"`
}

type SchemaType string

const (
	TypeString  SchemaType = "string"
	TypeNumber  SchemaType = "number"
	TypeInteger SchemaType = "integer"
	TypeBoolean SchemaType = "boolean"
	TypeArray   SchemaType = "array"
	TypeObject  SchemaType = "object"
)
