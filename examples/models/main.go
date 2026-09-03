// Command models lists the models the signed-in account can use, and prints
// the CLI's own version and authentication state.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := cursor.NewClientFromPath()
	if err != nil {
		log.Fatalf("locate cursor-agent: %v", err)
	}

	about, err := client.About(ctx)
	if err != nil {
		log.Fatalf("about: %v", err)
	}
	fmt.Printf("cli %s (%s), %s/%s, tier %s\n",
		about.CLIVersion, about.LatestStatus, about.OSPlatform, about.OSArch, about.SubscriptionTier)

	status, err := client.Status(ctx)
	if err != nil {
		log.Fatalf("status: %v", err)
	}
	// UserInfo carries personal data, so report presence rather than values.
	fmt.Printf("auth %s (account on file: %v)\n", status.Status, status.UserInfo.Email != "")

	models, err := client.Models(ctx)
	if err != nil {
		log.Fatalf("models: %v", err)
	}
	fmt.Printf("\n%d models available:\n", len(models))
	for _, m := range models {
		fmt.Printf("  %-40s %s\n", m.ID, m.Name)
	}
}
