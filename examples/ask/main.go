package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor"
)

func main() {
	client, err := cursor.NewClientFromPath()
	if err != nil {
		log.Fatal(err)
	}
	prompt := "Summarize README.md in one sentence."
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}
	result, err := client.Ask(prompt, &cursor.AskOptions{
		Model: "composer-2.5",
		Trust: true,
	})
	if result != nil && result.Result != "" {
		fmt.Println(result.Result)
	}
	if err != nil {
		log.Fatal(err)
	}
}
