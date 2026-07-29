package service

import (
	"context"
	"testing"
)

func TestResolveMentionsHereNotifiesAllActiveAgents(t *testing.T) {
	pool := taskSubmitTestPool(t)
	ctx := context.Background()
	ownerID := taskSubmitUser(t, pool)
	activeAgentID := taskSubmitAgent(t, pool, ownerID)
	secondAgentID := taskSubmitAgent(t, pool, ownerID)
	inactiveAgentID := taskSubmitAgent(t, pool, ownerID)
	nonMemberAgentID := taskSubmitAgent(t, pool, ownerID)
	channelID := taskSubmitChannel(t, pool, ownerID)
	taskSubmitMember(t, pool, channelID, "user", ownerID)
	taskSubmitMember(t, pool, channelID, "agent", activeAgentID)
	taskSubmitMember(t, pool, channelID, "agent", secondAgentID)
	taskSubmitMember(t, pool, channelID, "agent", inactiveAgentID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM channel_members WHERE channel_id = $1`, channelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM channels WHERE id = $1`, channelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE id IN ($1, $2, $3, $4)`, activeAgentID, secondAgentID, inactiveAgentID, nonMemberAgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, ownerID)
	})

	if _, err := pool.Exec(ctx, `UPDATE agents SET is_active = false WHERE id = $1`, inactiveAgentID); err != nil {
		t.Fatalf("deactivate agent: %v", err)
	}

	svc := NewMentionService(pool)

	contains := func(ids []string, id string) bool {
		for _, v := range ids {
			if v == id {
				return true
			}
		}
		return false
	}

	t.Run("here resolves all active channel agents", func(t *testing.T) {
		ids, hasMentions, err := svc.ResolveMentions(ctx, "hey @here please review this", channelID)
		if err != nil {
			t.Fatalf("ResolveMentions: %v", err)
		}
		if !hasMentions {
			t.Fatalf("expected hasMentions=true for @here")
		}
		if len(ids) != 2 {
			t.Fatalf("expected 2 agents, got %d: %v", len(ids), ids)
		}
		if !contains(ids, activeAgentID) || !contains(ids, secondAgentID) {
			t.Fatalf("missing active agents in %v", ids)
		}
		if contains(ids, inactiveAgentID) {
			t.Fatalf("inactive agent must not be notified: %v", ids)
		}
		if contains(ids, nonMemberAgentID) {
			t.Fatalf("non-member agent must not be notified: %v", ids)
		}
	})

	t.Run("here is case-insensitive", func(t *testing.T) {
		ids, _, err := svc.ResolveMentions(ctx, "@Here heads up", channelID)
		if err != nil {
			t.Fatalf("ResolveMentions: %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("expected 2 agents, got %d: %v", len(ids), ids)
		}
	})

	t.Run("here combined with named mention dedupes", func(t *testing.T) {
		var name string
		if err := pool.QueryRow(ctx, `SELECT name FROM agents WHERE id = $1`, activeAgentID).Scan(&name); err != nil {
			t.Fatalf("load agent name: %v", err)
		}
		ids, _, err := svc.ResolveMentions(ctx, "@here and @"+name, channelID)
		if err != nil {
			t.Fatalf("ResolveMentions: %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("expected deduped 2 agents, got %d: %v", len(ids), ids)
		}
	})

	t.Run("named mentions still work without here", func(t *testing.T) {
		var name string
		if err := pool.QueryRow(ctx, `SELECT name FROM agents WHERE id = $1`, activeAgentID).Scan(&name); err != nil {
			t.Fatalf("load agent name: %v", err)
		}
		ids, hasMentions, err := svc.ResolveMentions(ctx, "@"+name+" only you", channelID)
		if err != nil {
			t.Fatalf("ResolveMentions: %v", err)
		}
		if !hasMentions || len(ids) != 1 || ids[0] != activeAgentID {
			t.Fatalf("expected only the named agent, got %v (hasMentions=%v)", ids, hasMentions)
		}
	})
}
