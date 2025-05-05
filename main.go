package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/ollama/ollama/api"
)

func main() {
	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		fmt.Print("\u001b[94mYou\u001b[0m: ")
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	parsedURL, _ := url.Parse(baseURL)
	client := api.NewClient(parsedURL, http.DefaultClient)

	/* Defined tools in the tools.go */
	AllTools = []ToolDefinition{
		ReadFileDefinition,
		ListFilesDefinition,
		EditFileDefinition,
	}

	/* Convert AllTools to Ollama Api struct */
	tools := ConvertToolsToOllamaFormat(AllTools)

	// a bool variable to check if the user input needed
	readUserInput := true

	var history []api.Message
	fmt.Println(" Hello My Writer, welcome to Mulhim,")
	fmt.Println(" Where Writer get inspired.")

	for {
		// 	Check if the request needs a user input
		if readUserInput {
			userInput, ok := getUserMessage()
			if !ok {
				break
			}

			history = append(history, api.Message{
				Role:    "user",
				Content: userInput,
			})
		}
		resp, err := runOllamaChat(ctx, client, history, tools)
		if err != nil {
			log.Fatal(err)
		}
		history = append(history, resp.Message)
		if len(resp.Message.ToolCalls) > 0 {
			for _, call := range resp.Message.ToolCalls {
				fmt.Printf("\u001b[92mtool\u001b[0m: %s(%v)\n", call.Function.Name, call.Function.Arguments)
				toolResult := ExecuteTool(call.Function.Name, call.Function.Arguments)
				history = append(history, api.Message{Role: "tool", Content: toolResult})

			}
			readUserInput = false
		} else {
			fmt.Printf("\u001b[93mAsistant\u001b[0m: %s\n", resp.Message.Content)
			readUserInput = true
		}
	}
}

func runOllamaChat(ctx context.Context, client *api.Client, messages []api.Message, tools api.Tools) (*api.ChatResponse, error) {
	stream := false
	req := &api.ChatRequest{
		Model:    "qwen2.5:14b",
		Messages: messages,
		Tools:    tools,
		Stream:   &stream,
	}
	var result *api.ChatResponse
	err := client.Chat(ctx, req, func(resp api.ChatResponse) error {
		result = &resp
		return nil
	})
	return result, err
}
