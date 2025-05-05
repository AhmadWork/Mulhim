package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ollama/ollama/api"
)

/*
This is the main struct that we will be using for creating every tool
*/
type ToolDefinition struct {
	Name        string
	Desc        string
	InputSchema map[string]interface{}
	Function    func(input json.RawMessage) (string, error)
}

/*The array that will be holding all the tools we build in */
var AllTools = []ToolDefinition{}

/*
A helper function that take array of ToolDefinition and convert them
to Ollama Api structure.
*/
func ConvertToolsToOllamaFormat(tools []ToolDefinition) api.Tools {
	type flexibleTool struct {
		Type     string `json:"type"`
		Function struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			Parameters  map[string]interface{} `json:"parameters"`
		} `json:"function"`
	}

	var temp []flexibleTool

	for _, t := range tools {
		f := flexibleTool{
			Type: "function",
		}
		f.Function.Name = t.Name
		f.Function.Description = t.Desc

		// Build parameters object with enhanced error handling
		properties := make(map[string]interface{})
		required := make([]string, 0, len(t.InputSchema))

		for key, val := range t.InputSchema {
			// Safely type assert to handle potential panic
			field, ok := val.(map[string]interface{})
			if !ok {
				// Skip invalid schema entries
				continue
			}

			// Safely extract type and description
			fieldType, _ := field["type"].(string)
			fieldDesc, _ := field["description"].(string)

			properties[key] = map[string]interface{}{
				"type":        fieldType,
				"description": fieldDesc,
			}
			required = append(required, key)
		}

		f.Function.Parameters = map[string]interface{}{
			"type":       "object",
			"required":   required,
			"properties": properties,
		}

		temp = append(temp, f)
	}

	// Marshal to JSON and decode into api.Tools with error logging
	raw, err := json.Marshal(temp)
	if err != nil {
		panic(fmt.Errorf("failed to marshal flexible tools: %w", err))
	}

	var toolsList api.Tools
	if err := json.Unmarshal(raw, &toolsList); err != nil {
		panic(fmt.Errorf("failed to unmarshal into api.Tools: %w", err))
	}

	return toolsList
}

/*
A helper function that will use to Excute a tool if It exist
and return It's output to the LLM.
*/

func ExecuteTool(name string, args map[string]interface{}) string {
	// Convert args to JSON input
	input, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("error marshaling input: %v", err)
	}
	// Find and execute the matching tool
	for _, tool := range AllTools {
		if tool.Name == name {
			result, err := tool.Function(input)
			if err != nil {
				return fmt.Sprintf("error: %v", err)
			}
			return result
		}
	}
	return "tool not found"
}

/*Read file tool definition*/
var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Desc:        "Reads a file from the local file system Only excite this if a file path is given",
	InputSchema: map[string]interface{}{"path": map[string]interface{}{"type": "string", "description": "Relative path to the file"}},
	Function:    readFile,
}

/*the function will be using to read file if the tool read_file is called*/
func readFile(input json.RawMessage) (string, error) {
	var data struct{ Path string }
	if err := json.Unmarshal(input, &data); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Validate path before reading
	if data.Path == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	content, err := os.ReadFile(data.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", data.Path, err)
	}
	return string(content), nil
}
