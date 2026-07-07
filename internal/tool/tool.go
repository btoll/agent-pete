package tool

import (
	"os"
)

//  "tools": [ //    {
//      "type": "function",
//      "function": {
//        "name": "get_weather",
//        "description": "Get the current weather in a given location",
//        "parameters": {
//          "type": "object",
//          "properties": {
//            "location": {
//              "type": "string",
//              "description": "The city and state, e.g. San Francisco, CA"
//            }
//          },
//          "required": ["location"]
//        }
//      }
//    }
//  ]

type Tool struct {
	Type     string
	Function Function
}

type Function struct {
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
		Function: Function{
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
		Function: Function{
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
