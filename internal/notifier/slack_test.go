package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/heyjobs/terranovate/internal/version"
)

func TestNew(t *testing.T) {
	webhookURL := "https://hooks.slack.com/test"
	channel := "#terraform"

	notifier := New(webhookURL, channel)

	if notifier == nil {
		t.Fatal("New() returned nil")
	}

	if notifier.slackWebhookURL != webhookURL {
		t.Errorf("slackWebhookURL = %s, want %s", notifier.slackWebhookURL, webhookURL)
	}

	if notifier.slackChannel != channel {
		t.Errorf("slackChannel = %s, want %s", notifier.slackChannel, channel)
	}

	if notifier.httpClient == nil {
		t.Error("httpClient is nil")
	}
}

func TestSendSlack(t *testing.T) {
	tests := []struct {
		name           string
		webhookURL     string
		data           NotificationData
		serverResponse int
		wantErr        bool
		validateReq    func(*testing.T, *http.Request, map[string]interface{})
	}{
		{
			name:       "successful send",
			webhookURL: "", // will be set to test server URL
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:   "vpc",
							Source: "terraform-aws-modules/vpc/aws",
						},
						CurrentVersion: "4.0.0",
						LatestVersion:  "5.0.0",
						IsOutdated:     true,
						UpdateType:     version.UpdateTypeMajor,
					},
				},
				TotalUpdates: 1,
				Timestamp:    time.Now(),
			},
			serverResponse: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, r *http.Request, body map[string]interface{}) {
				// Verify content type
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %s, want application/json", ct)
				}

				// Verify message contains expected text
				if text, ok := body["text"].(string); ok {
					if !strings.Contains(text, "1 module update") {
						t.Errorf("Message text does not contain expected update count: %s", text)
					}
				} else {
					t.Error("Message does not contain text field")
				}

				// Verify attachments exist
				if attachments, ok := body["attachments"].([]interface{}); ok {
					if len(attachments) != 1 {
						t.Errorf("Expected 1 attachment, got %d", len(attachments))
					}
				}
			},
		},
		{
			name:       "server error",
			webhookURL: "",
			data: NotificationData{
				Updates:      []version.UpdateInfo{},
				TotalUpdates: 0,
				Timestamp:    time.Now(),
			},
			serverResponse: http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:       "no webhook URL",
			webhookURL: "",
			data:       NotificationData{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedReq *http.Request
			var receivedBody map[string]interface{}

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedReq = r

				// Parse body
				if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				w.WriteHeader(tt.serverResponse)
			}))
			defer server.Close()

			webhookURL := tt.webhookURL
			if tt.name != "no webhook URL" {
				webhookURL = server.URL
			}

			notifier := New(webhookURL, "#test")
			err := notifier.SendSlack(context.Background(), tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendSlack() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.validateReq != nil && receivedReq != nil {
				tt.validateReq(t, receivedReq, receivedBody)
			}
		})
	}
}

func TestOutputJSON(t *testing.T) {
	notifier := New("", "")

	data := NotificationData{
		Updates: []version.UpdateInfo{
			{
				Module: scanner.ModuleInfo{
					Name:   "vpc",
					Source: "terraform-aws-modules/vpc/aws",
				},
				CurrentVersion: "4.0.0",
				LatestVersion:  "5.0.0",
				IsOutdated:     true,
			},
		},
		TotalUpdates: 1,
		Timestamp:    time.Now(),
	}

	output, err := notifier.OutputJSON(data)
	if err != nil {
		t.Fatalf("OutputJSON() error = %v", err)
	}

	if output == "" {
		t.Error("OutputJSON() returned empty string")
	}

	// Verify it's valid JSON
	var parsed NotificationData
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("OutputJSON() did not produce valid JSON: %v", err)
	}

	if parsed.TotalUpdates != 1 {
		t.Errorf("Parsed JSON TotalUpdates = %d, want 1", parsed.TotalUpdates)
	}
}

func TestOutputText(t *testing.T) {
	tests := []struct {
		name         string
		data         NotificationData
		wantContains []string
		wantNotContain []string
	}{
		{
			name: "no updates",
			data: NotificationData{
				Updates:      []version.UpdateInfo{},
				TotalUpdates: 0,
			},
			wantContains: []string{"No module updates available"},
		},
		{
			name: "single update without breaking change",
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:     "vpc",
							Source:   "terraform-aws-modules/vpc/aws",
							FilePath: "/project/main.tf",
							Line:     10,
						},
						CurrentVersion: "4.0.0",
						LatestVersion:  "4.1.0",
						IsOutdated:     true,
						UpdateType:     version.UpdateTypeMinor,
					},
				},
				TotalUpdates: 1,
			},
			wantContains: []string{
				"Found 1 module update(s)",
				"vpc",
				"terraform-aws-modules/vpc/aws",
				"4.0.0",
				"4.1.0",
				"main.tf:10",
				"minor",
			},
			wantNotContain: []string{"breaking"},
		},
		{
			name: "update with breaking change",
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:     "vpc",
							Source:   "terraform-aws-modules/vpc/aws",
							FilePath: "/project/main.tf",
							Line:     10,
						},
						CurrentVersion:        "4.0.0",
						LatestVersion:         "5.0.0",
						IsOutdated:            true,
						HasBreakingChange:     true,
						BreakingChangeDetails: "Major version upgrade",
						UpdateType:            version.UpdateTypeMajor,
						ChangelogURL:          "https://example.com/changelog",
					},
				},
				TotalUpdates: 1,
			},
			wantContains: []string{
				"Found 1 module update(s)",
				"(1 with breaking changes)",
				"⚠️",
				"BREAKING CHANGE",
				"Major version upgrade",
				"https://example.com/changelog",
			},
		},
		{
			name: "multiple updates",
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:   "vpc",
							Source: "terraform-aws-modules/vpc/aws",
						},
						CurrentVersion: "4.0.0",
						LatestVersion:  "5.0.0",
						IsOutdated:     true,
					},
					{
						Module: scanner.ModuleInfo{
							Name:   "eks",
							Source: "terraform-aws-modules/eks/aws",
						},
						CurrentVersion: "19.0.0",
						LatestVersion:  "19.1.0",
						IsOutdated:     true,
					},
				},
				TotalUpdates: 2,
			},
			wantContains: []string{
				"Found 2 module update(s)",
				"vpc",
				"eks",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := New("", "")
			output := notifier.OutputText(tt.data)

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("OutputText() output does not contain %q\nOutput:\n%s", want, output)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("OutputText() output should not contain %q\nOutput:\n%s", notWant, output)
				}
			}
		})
	}
}

func TestOutputMarkdown(t *testing.T) {
	tests := []struct {
		name         string
		data         NotificationData
		wantContains []string
	}{
		{
			name: "no updates",
			data: NotificationData{
				Updates:         []version.UpdateInfo{},
				ProviderUpdates: []version.ProviderUpdateInfo{},
				TotalUpdates:    0,
				Timestamp:       time.Now(),
			},
			wantContains: []string{
				"Terranovate Dependency Check",
				"All modules and providers are up to date",
			},
		},
		{
			name: "module updates",
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:     "vpc",
							Source:   "terraform-aws-modules/vpc/aws",
							FilePath: "/project/main.tf",
							Line:     10,
						},
						CurrentVersion: "4.0.0",
						LatestVersion:  "5.0.0",
						IsOutdated:     true,
						UpdateType:     version.UpdateTypeMajor,
						ChangelogURL:   "https://example.com/changelog",
					},
				},
				TotalUpdates: 1,
				Timestamp:    time.Now(),
			},
			wantContains: []string{
				"## 🔍 Terranovate Dependency Check",
				"### 📦 Module Updates",
				"<details>",
				"<strong>1. vpc</strong>",
				"`4.0.0`",
				"`5.0.0`",
				"[View](https://example.com/changelog)",
				"Generated by",
			},
		},
		{
			name: "provider updates",
			data: NotificationData{
				ProviderUpdates: []version.ProviderUpdateInfo{
					{
						Provider: scanner.ProviderInfo{
							Name:     "aws",
							Source:   "hashicorp/aws",
							FilePath: "/project/versions.tf",
							Line:     5,
						},
						CurrentVersion: "4.0.0",
						LatestVersion:  "5.0.0",
						IsOutdated:     true,
						UpdateType:     version.UpdateTypeMajor,
					},
				},
				TotalUpdates: 1,
				Timestamp:    time.Now(),
			},
			wantContains: []string{
				"### 🔌 Provider Updates",
				"<strong>1. aws</strong>",
			},
		},
		{
			name: "breaking change warning",
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:   "vpc",
							Source: "terraform-aws-modules/vpc/aws",
						},
						CurrentVersion:        "4.0.0",
						LatestVersion:         "5.0.0",
						IsOutdated:            true,
						HasBreakingChange:     true,
						BreakingChangeDetails: "Major version upgrade may contain breaking changes",
						UpdateType:            version.UpdateTypeMajor,
					},
				},
				TotalUpdates: 1,
				Timestamp:    time.Now(),
			},
			wantContains: []string{
				"⚠️ **Warning**",
				"Breaking Change",
				"Action Required",
				"Review changelogs",
			},
		},
		{
			name: "resource changes",
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:   "vpc",
							Source: "terraform-aws-modules/vpc/aws",
						},
						CurrentVersion: "4.0.0",
						LatestVersion:  "5.0.0",
						ResourceChanges: &version.ResourceChangesSummary{
							HasChanges:     true,
							TotalReplace:   2,
							TotalDelete:    1,
							TotalModify:    3,
							ResourcesToReplace: []version.ResourceChange{
								{
									Address: "aws_vpc.main",
									Reason:  "attribute change forces replacement",
								},
							},
							ResourcesToDelete: []version.ResourceChange{
								{
									Address: "aws_subnet.old",
								},
							},
						},
					},
				},
				TotalUpdates: 1,
				Timestamp:    time.Now(),
			},
			wantContains: []string{
				"Resource Changes",
				"2 resource(s) will be REPLACED",
				"1 resource(s) will be DELETED",
				"3 resource(s) will be MODIFIED",
				"`aws_vpc.main`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := New("", "")
			output := notifier.OutputMarkdown(tt.data)

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("OutputMarkdown() output does not contain %q\nOutput:\n%s", want, output)
				}
			}
		})
	}
}

func TestBuildSlackMessage(t *testing.T) {
	tests := []struct {
		name     string
		data     NotificationData
		channel  string
		validate func(*testing.T, map[string]interface{})
	}{
		{
			name: "basic message",
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:   "vpc",
							Source: "terraform-aws-modules/vpc/aws",
						},
						CurrentVersion: "4.0.0",
						LatestVersion:  "5.0.0",
						UpdateType:     version.UpdateTypeMajor,
					},
				},
				TotalUpdates: 1,
				Timestamp:    time.Now(),
			},
			channel: "#terraform",
			validate: func(t *testing.T, msg map[string]interface{}) {
				if text, ok := msg["text"].(string); !ok || text == "" {
					t.Error("Message does not contain text field")
				}

				if channel, ok := msg["channel"].(string); !ok || channel != "#terraform" {
					t.Errorf("Channel = %v, want #terraform", channel)
				}

				if attachments, ok := msg["attachments"].([]map[string]interface{}); !ok || len(attachments) == 0 {
					t.Error("Message does not contain attachments")
				}
			},
		},
		{
			name: "breaking change",
			data: NotificationData{
				Updates: []version.UpdateInfo{
					{
						Module: scanner.ModuleInfo{
							Name:   "vpc",
							Source: "terraform-aws-modules/vpc/aws",
						},
						CurrentVersion:    "4.0.0",
						LatestVersion:     "5.0.0",
						HasBreakingChange: true,
						UpdateType:        version.UpdateTypeMajor,
					},
				},
				TotalUpdates: 1,
				Timestamp:    time.Now(),
			},
			validate: func(t *testing.T, msg map[string]interface{}) {
				text := msg["text"].(string)
				if !strings.Contains(text, "⚠️") {
					t.Error("Message text does not contain warning emoji for breaking change")
				}
				if !strings.Contains(text, "breaking changes") {
					t.Error("Message text does not mention breaking changes")
				}

				attachments := msg["attachments"].([]map[string]interface{})
				if len(attachments) > 0 {
					if color := attachments[0]["color"]; color != "danger" {
						t.Errorf("Attachment color = %v, want danger", color)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := New("", tt.channel)
			msg := notifier.buildSlackMessage(tt.data)

			if tt.validate != nil {
				tt.validate(t, msg)
			}
		})
	}
}

func TestSendSlack_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := New(server.URL, "")

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	data := NotificationData{
		Updates:      []version.UpdateInfo{},
		TotalUpdates: 0,
		Timestamp:    time.Now(),
	}

	err := notifier.SendSlack(ctx, data)
	if err == nil {
		t.Error("SendSlack() expected error with cancelled context, got nil")
	}
}
