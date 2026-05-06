package main

import (
	"os"
	"strings"
	"testing"
)

func TestSearchUIIncludesProgramGuideViewToggle(t *testing.T) {
	content, err := os.ReadFile("static/search.html")
	if err != nil {
		t.Fatalf("failed to read search UI: %v", err)
	}

	html := string(content)
	requiredSnippets := []string{
		`id="viewToggleButton"`,
		`id="guideView"`,
		`id="guideTopScrollbar"`,
		`id="guideMainScroll"`,
		`function renderGuideView(programs)`,
		`function toggleResultsView()`,
		`function syncGuideScrollbarWidth()`,
		`function setupGuideCardInteractions()`,
		`id="programDetailModal"`,
		`id="reservationModal" class="fixed inset-0 z-60`,
		`id="autoReservationModal" class="fixed inset-0 z-60`,
		`function openGuideProgramDetails(programData)`,
		`function closeProgramDetailModal()`,
		`function handleProgramDetailReserve()`,
		`function supportsGuideHoverLock()`,
		`function classifyGuideCardDensity(cardHeight)`,
		`.guide-card-hover-panel`,
		`.guide-hover-locked .guide-program-card`,
		`data-program="${encodeURIComponent(JSON.stringify(program))}"`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(html, snippet) {
			t.Fatalf("search UI is missing required snippet: %s", snippet)
		}
	}
}
