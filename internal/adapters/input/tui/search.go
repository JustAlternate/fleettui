package tui

import (
	"strings"

	"github.com/justalternate/fleetui/internal/domain"
)

// getDisplayedNodes returns the filtered node list when a search is active,
// otherwise the full node list.
func (m *Model) getDisplayedNodes() []*domain.Node {
	if m.searchText != "" {
		return m.filteredNodes
	}
	return m.nodes
}

// applyFilter recomputes filteredNodes based on the current search text.
// The query is matched case-insensitively against node name, status, and IP.
// A leading "!" negates the match.
func (m *Model) applyFilter() {
	query := strings.TrimSpace(m.searchText)
	if query == "" {
		m.filteredNodes = nil
		return
	}

	negate := false
	if strings.HasPrefix(query, "!") {
		negate = true
		query = strings.TrimPrefix(query, "!")
		query = strings.TrimSpace(query)
	}
	lowerQuery := strings.ToLower(query)

	var result []*domain.Node
	for _, node := range m.nodes {
		haystack := strings.ToLower(node.Name + " " + nodeStatusString(node) + " " + node.IP)
		matched := strings.Contains(haystack, lowerQuery)
		if negate {
			matched = !matched
		}
		if matched {
			result = append(result, node)
		}
	}

	m.filteredNodes = result

	// Clamp cursor to filtered list size.
	displayed := m.getDisplayedNodes()
	if m.cursor >= len(displayed) && len(displayed) > 0 {
		m.cursor = len(displayed) - 1
	}
}

// nodeStatusString returns a lowercase status label for a node.
func nodeStatusString(node *domain.Node) string {
	switch {
	case node.IsPending():
		return "pending"
	case !node.IsAvailable():
		if node.Error != "" {
			return "error"
		}
		return "offline"
	default:
		return "online"
	}
}

// renderSearchBar renders the search/filter input line.
func (m *Model) renderSearchBar() string {
	if m.searchMode {
		// Show active text input.
		return SearchBarStyle.Render(m.searchInput.View())
	}

	// Show read-only filter summary.
	filterText := m.searchText
	label := SearchFilterInfoStyle.Render("Filter: ")
	query := SearchTextStyle.Render(filterText)
	clear := SearchFilterInfoStyle.Render(" [esc] clear")
	return SearchBarStyle.Render(label + query + clear)
}
