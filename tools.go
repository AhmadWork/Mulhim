package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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

/*Read files list (Basically ls) tool definition*/
var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Desc:        "Lists files in a directory. Only use this when the user specifically asks for a list of files. If no path is given, assume the current directory.",
	InputSchema: map[string]interface{}{"path": map[string]interface{}{"type": "string", "description": "Directory path to list files from"}},
	Function:    listFiles,
}

/*the function will be using to list files if the tool list_files is called*/
func listFiles(input json.RawMessage) (string, error) {
	var data struct{ Path string }
	if err := json.Unmarshal(input, &data); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Default to current directory if no path provided
	dir := "."
	if data.Path != "" {
		dir = data.Path
	}

	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == dir {
			return nil
		}

		// Get relative path
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Distinguish between directories and files
		if info.IsDir() {
			files = append(files, rel+"/")
		} else {
			files = append(files, rel)
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error listing files in %s: %w", dir, err)
	}

	// Convert to JSON
	b, err := json.Marshal(files)
	if err != nil {
		return "", fmt.Errorf("error converting files to JSON: %w", err)
	}

	return string(b), nil
}

/*Edit file tool definition*/
var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Desc: "Replaces text in a file. Only use this when the user specifies a file path and provides old and new text. if the file does not exist and old_str is not provided just create a new file with the new_str",
	InputSchema: map[string]interface{}{
		"path":    map[string]interface{}{"type": "string", "description": "Path to the file"},
		"old_str": map[string]interface{}{"type": "string", "description": "Text to be replaced"},
		"new_str": map[string]interface{}{"type": "string", "description": "Replacement text"},
	},
	Function: editFile,
}

/*the function will be using to edit file if the tool edit_file is called*/
func editFile(input json.RawMessage) (string, error) {
	var data struct {
		Path   string `json:"path"`
		OldStr string `json:"old_str"`
		NewStr string `json:"new_str"`
	}
	if err := json.Unmarshal(input, &data); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Validate inputs
	if data.Path == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	content, err := os.ReadFile(data.Path)
	if err != nil {
		if os.IsNotExist(err) && data.OldStr == "" {
			return createNewFile(data.Path, data.NewStr)
		}
		return "", err
	}

	if data.OldStr == "" {
		return "", fmt.Errorf("old text cannot be empty")
	}
	if data.OldStr == data.NewStr {
		return "", fmt.Errorf("old and new text must be different")
	}

	// Replace text
	updated := strings.Replace(string(content), data.OldStr, data.NewStr, -1)

	// Write updated content
	if err := os.WriteFile(data.Path, []byte(updated), 0644); err != nil {
		return "", fmt.Errorf("failed to write updated file %s: %w", data.Path, err)
	}

	return "OK", nil
}

/*A helper function that the editFile use if the file does not exist*/
func createNewFile(filePath, content string) (string, error) {
	dir := path.Dir(filePath)
	if dir != "." {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	return fmt.Sprintf("Successfully created file %s", filePath), nil
}
