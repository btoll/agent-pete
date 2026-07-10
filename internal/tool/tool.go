package tool

import (
	"os"
)

type Tool struct {
	Type     string
	Function ToolFunction
}

type ToolFunction struct {
	Name        string
	Description string
	Parameters  ParameterSchema
}

type ParameterSchema struct {
	Type       string
	Properties map[string]any
	Required   []string
}

var Tools = map[string]Tool{
	"Add": {
		Type: "function",
		Function: ToolFunction{
			Name:        "Add",
			Description: "Adds two numbers",
			Parameters: ParameterSchema{
				Type: "object",
				Properties: map[string]any{
					"a": map[string]any{
						"type":        "number",
						"description": "The left operand",
					},
					"b": map[string]any{
						"type":        "number",
						"description": "The right operand",
					},
				},
				Required: []string{"a", "b"},
			},
		},
	},
	"ReadFile": {
		Type: "function",
		Function: ToolFunction{
			Name:        "ReadFile",
			Description: "Opens a file and reads it",
			Parameters: ParameterSchema{
				Type: "object",
				Properties: map[string]any{
					"filename": map[string]any{
						"type":        "string",
						"description": "The file to read",
					},
				},
				Required: []string{"filename"},
			},
		},
	},
}

func Add[T int | float64](a, b T) T {
	return a + b
}

func ReadFile(filename string) (string, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
