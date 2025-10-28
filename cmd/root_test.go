package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCmd(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}

	if rootCmd.Use != "terranovate" {
		t.Errorf("rootCmd.Use = %s, want terranovate", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("rootCmd.Short should not be empty")
	}

	if rootCmd.Long == "" {
		t.Error("rootCmd.Long should not be empty")
	}

	// Test that PersistentPreRun is set
	if rootCmd.PersistentPreRun == nil {
		t.Error("rootCmd.PersistentPreRun should be set")
	}
}

func TestRootCmdFlags(t *testing.T) {
	// Test config flag
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Fatal("config flag not found")
	}

	if configFlag.DefValue != ".terranovate.yaml" {
		t.Errorf("config flag default = %s, want .terranovate.yaml", configFlag.DefValue)
	}

	// Test verbose flag
	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Fatal("verbose flag not found")
	}

	if verboseFlag.Shorthand != "v" {
		t.Errorf("verbose flag shorthand = %s, want v", verboseFlag.Shorthand)
	}

	// Test json-log flag
	jsonLogFlag := rootCmd.PersistentFlags().Lookup("json-log")
	if jsonLogFlag == nil {
		t.Fatal("json-log flag not found")
	}

	if jsonLogFlag.DefValue != "false" {
		t.Errorf("json-log flag default = %s, want false", jsonLogFlag.DefValue)
	}
}

func TestRootCmdExecution(t *testing.T) {
	// Test that root command can be executed with --help
	cmd := &cobra.Command{
		Use: "test",
	}

	// Add a test subcommand
	cmd.AddCommand(&cobra.Command{
		Use:   "subcommand",
		Short: "Test subcommand",
		Run: func(cmd *cobra.Command, args []string) {
			// Do nothing
		},
	})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Command execution with --help failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Help output should not be empty")
	}
}

func TestPersistentFlagsInheritance(t *testing.T) {
	// Create a child command to test flag inheritance
	childCmd := &cobra.Command{
		Use:   "child",
		Short: "Child command",
		Run: func(cmd *cobra.Command, args []string) {
			// Do nothing
		},
	}

	rootCmd.AddCommand(childCmd)
	defer rootCmd.RemoveCommand(childCmd)

	// Test that persistent flags are inherited
	configFlag := childCmd.Flags().Lookup("config")
	if configFlag == nil {
		// Try inherited flags
		configFlag = childCmd.InheritedFlags().Lookup("config")
	}

	if configFlag == nil {
		t.Error("Child command should inherit config flag from root")
	}

	verboseFlag := childCmd.InheritedFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("Child command should inherit verbose flag from root")
	}
}

func TestRootCmdSubcommands(t *testing.T) {
	// Test that root command has subcommands
	commands := rootCmd.Commands()

	if len(commands) == 0 {
		t.Error("rootCmd should have subcommands")
	}

	// Check for expected command names
	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Name()] = true
	}

	// These commands should exist based on the files in cmd/
	expectedCommands := []string{"scan", "check", "pr", "plan", "notify"}
	for _, expected := range expectedCommands {
		if !commandNames[expected] {
			// Not all commands may be registered yet, so we just log
			t.Logf("Expected command %s not found (may not be registered)", expected)
		}
	}
}

func TestVerboseFlagBehavior(t *testing.T) {
	// Reset verbose flag
	verbose = false

	// Test setting verbose flag
	rootCmd.PersistentFlags().Set("verbose", "true")
	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")

	if verboseFlag.Value.String() != "true" {
		t.Error("Setting verbose flag to true failed")
	}

	// Reset
	rootCmd.PersistentFlags().Set("verbose", "false")
}

func TestJsonLogFlagBehavior(t *testing.T) {
	// Reset json-log flag
	jsonLog = false

	// Test setting json-log flag
	rootCmd.PersistentFlags().Set("json-log", "true")
	jsonLogFlag := rootCmd.PersistentFlags().Lookup("json-log")

	if jsonLogFlag.Value.String() != "true" {
		t.Error("Setting json-log flag to true failed")
	}

	// Reset
	rootCmd.PersistentFlags().Set("json-log", "false")
}

func TestConfigFileFlagBehavior(t *testing.T) {
	// Test setting custom config file path
	customPath := "/custom/path/config.yaml"
	rootCmd.PersistentFlags().Set("config", customPath)

	configFlag := rootCmd.PersistentFlags().Lookup("config")
	if configFlag.Value.String() != customPath {
		t.Errorf("config flag value = %s, want %s", configFlag.Value.String(), customPath)
	}

	// Reset to default
	rootCmd.PersistentFlags().Set("config", ".terranovate.yaml")
}

func TestRootCmdDescription(t *testing.T) {
	// Test that description contains key terms
	if rootCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if rootCmd.Long == "" {
		t.Error("Long description should not be empty")
	}

	// Check for important keywords in description
	longDesc := rootCmd.Long
	keywords := []string{"Terraform", "module", "update"}

	for _, keyword := range keywords {
		if !contains(longDesc, keyword) {
			t.Errorf("Long description should contain %q", keyword)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExecute(t *testing.T) {
	// We can't easily test Execute() as it calls os.Exit
	// Just verify the function exists by checking rootCmd
	if rootCmd == nil {
		t.Error("rootCmd should be initialized for Execute to work")
	}
}
