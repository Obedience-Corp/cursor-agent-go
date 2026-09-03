package cursor

import (
	"context"
	"testing"
)

func TestAboutDecodes(t *testing.T) {
	client := mockClient(t, "ask-success")
	about, err := client.About(context.Background())
	if err != nil {
		t.Fatalf("About: %v", err)
	}
	if about.CLIVersion != TestedAgentVersion {
		t.Fatalf("cliVersion = %q, want %q", about.CLIVersion, TestedAgentVersion)
	}
	if !about.IsUpToDate() {
		t.Fatalf("IsUpToDate = false for latestStatus %q", about.LatestStatus)
	}
	if about.OSArch != "arm64" || about.SubscriptionTier != "Ultra" {
		t.Fatalf("about = %+v", about)
	}
}

func TestStatusDecodes(t *testing.T) {
	client := mockClient(t, "ask-success")
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.IsAuthenticated || status.Status != "authenticated" {
		t.Fatalf("status = %+v", status)
	}
	if !status.HasAccessToken || !status.HasRefreshToken {
		t.Fatalf("token flags = %+v", status)
	}
	if status.UserInfo.Email != "tester@example.com" || status.UserInfo.UserID != 1 {
		t.Fatalf("userInfo = %+v", status.UserInfo)
	}
	if len(status.Raw) == 0 {
		t.Fatal("Raw was not preserved")
	}
}

func TestModelsParsesTextListing(t *testing.T) {
	client := mockClient(t, "ask-success")
	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("got %d models, want 3: %+v", len(models), models)
	}
	if models[0].ID != "auto" || models[0].Name != "Auto (default)" {
		t.Fatalf("first model = %+v", models[0])
	}
	if models[2].ID != "composer-2.5" {
		t.Fatalf("third model = %+v", models[2])
	}
	// The "Available models" header must not be parsed as an entry.
	for _, m := range models {
		if m.ID == "Available" {
			t.Fatal("header line was parsed as a model")
		}
	}
}

func TestAdminHonorsCanceledContext(t *testing.T) {
	client := mockClient(t, "ask-success")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.About(ctx); err == nil {
		t.Fatal("About: expected error for canceled context")
	} else {
		requireCursorError(t, err, KindInterrupted)
	}
	if _, err := client.Status(ctx); err == nil {
		t.Fatal("Status: expected error for canceled context")
	}
	if _, err := client.Models(ctx); err == nil {
		t.Fatal("Models: expected error for canceled context")
	}
}
