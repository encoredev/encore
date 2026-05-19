//go:build e2e

package mcp

import (
	"testing"
)

// TestWaitForSubscriptionMessageE2E exercises the wait_for_subscription_message
// MCP tool against a real Encore app fixture.
//
// Run with: go test -tags=e2e ./cli/daemon/mcp/ -run TestWaitForSubscriptionMessageE2E -v
//
// This is currently a scaffold — wiring requires either:
//  1. Booting the daemon in-process and connecting via SSE, or
//  2. Shelling out to `encore daemon -f` and an `encore mcp run` client.
//
// The unit tests in pubsub_wait_test.go cover the broker, match filter,
// span detail extraction, wait orchestration, timeout, since-watermark,
// and topic/subscription validation. Manual smoke testing of the MCP
// tool happens out-of-band against a live fixture; see the plan at
// docs/superpowers/plans/2026-05-08-encore-mcp-pubsub-improvements.md
// (Task A9 Step 3) for the manual test recipe.
func TestWaitForSubscriptionMessageE2E(t *testing.T) {
	t.Skip("e2e wiring deferred; unit tests cover the core; see plan A9 for manual smoke test recipe")
}
