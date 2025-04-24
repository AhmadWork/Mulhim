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

	var history []api.Message
	for {
		userInput, ok := getUserMessage()
		if !ok {
			break
		}

		history = append(history, api.Message{
			Role:    "user",
			Content: userInput,
		})
		resp, err := runOllamaChat(ctx, client, history)
		if err != nil {
			log.Fatal(err)
		}
		history = append(history, resp.Message)
		fmt.Printf("\u001b[93mAsistant\u001b[0m: %s\n", resp.Message.Content)
	}
}

func runOllamaChat(ctx context.Context, client *api.Client, messages []api.Message) (*api.ChatResponse, error) {
	stream := false
	req := &api.ChatRequest{
		Model:    "qwen2.5:14b",
		Messages: messages,
		Stream:   &stream,
	}
	var result *api.ChatResponse
	err := client.Chat(ctx, req, func(resp api.ChatResponse) error {
		result = &resp
		return nil
	})
	return result, err
}

