package main

import (
	"os"
	"strings"
	"testing"
)

func TestClaudeMdSymlinksToAgentsMd(t *testing.T) {
	info, err := os.Lstat("CLAUDE.md")
	if err != nil {
		t.Fatalf("failed to stat CLAUDE.md: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("CLAUDE.md is not a symlink")
	}

	target, err := os.Readlink("CLAUDE.md")
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md symlink: %v", err)
	}
	if target != "AGENTS.md" {
		t.Fatalf("CLAUDE.md should point to AGENTS.md, got %q", target)
	}
}

func TestAgentsMdIncludesActiveClaudeGuidance(t *testing.T) {
	content, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}

	text := string(content)
	requiredSnippets := []string{
		"Respond to collaborators in Japanese, but keep code comments in English.",
		"Prefer real integrations and production behavior.",
		"Write or update tests before implementation when practical.",
		"Use `gh` when GitHub state or review context is relevant.",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(text, snippet) {
			t.Fatalf("AGENTS.md is missing required guidance: %s", snippet)
		}
	}
}
