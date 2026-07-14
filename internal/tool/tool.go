package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	Type                 string
	Properties           map[string]any
	Required             []string
	AdditionalProperties bool
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
				Required:             []string{"a", "b"},
				AdditionalProperties: false,
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
				Required:             []string{"filename"},
				AdditionalProperties: false,
			},
		},
	},
	"WriteFile": {
		Type: "function",
		Function: ToolFunction{
			Name:        "WriteFile",
			Description: "Writes data to a file. Will create the file if it doesn't exist.",
			Parameters: ParameterSchema{
				Type: "object",
				Properties: map[string]any{
					"filename": map[string]any{
						"type":        "string",
						"description": "The file to write to",
					},
					"data": map[string]any{
						"type":        "string",
						"description": "The data to write to the file",
					},
				},
				Required:             []string{"filename", "data"},
				AdditionalProperties: false,
			},
		},
	},
}

func Add[T int | float64](a, b T) T {
	return a + b
}

func ReadFile(filename string) (string, error) {
	dir, file := filepath.Split(filename)
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dirs := filepath.Join(cwd, dir)
	b, err := os.ReadFile(filepath.Join(dirs, file))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func WriteFile(filename string, data string) (string, error) {
	dir, file := filepath.Split(filename)
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dirs := filepath.Join(cwd, dir)
	err = os.MkdirAll(dirs, 0755)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(filepath.Join(dirs, file), []byte(strings.ReplaceAll(data, "\\n", "\n")), 0644)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("created filename `%s`", file), nil
}
