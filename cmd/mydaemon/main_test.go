package main

import "testing"

func TestParseCommandDefaultsToForegroundFalse(t *testing.T) {
	parsed, err := parseArgs([]string{"start"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if parsed.command != "start" {
		t.Fatalf("expected start, got %q", parsed.command)
	}
	if parsed.foreground {
		t.Fatalf("expected foreground false")
	}
}

func TestParseArgsRejectsUnknownCommand(t *testing.T) {
	_, err := parseArgs([]string{"launch"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestParseArgsAcceptsImplicitStartWithForeground(t *testing.T) {
	parsed, err := parseArgs([]string{"--foreground"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if parsed.command != "start" {
		t.Fatalf("expected implicit start, got %q", parsed.command)
	}
	if !parsed.foreground {
		t.Fatalf("expected foreground true")
	}
}

func TestParseArgsRejectsMultipleCommands(t *testing.T) {
	_, err := parseArgs([]string{"start", "status"})
	if err == nil {
		t.Fatal("expected error for multiple commands")
	}
}
