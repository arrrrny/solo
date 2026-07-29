package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestParseMentionNames(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "no mentions", content: "hello everyone", want: nil},
		{name: "single mention", content: "hey @lucy ping", want: []string{"lucy"}},
		{name: "here keyword", content: "@here standup time", want: []string{"here"}},
		{name: "here with comma", content: "@here, please review", want: []string{"here"}},
		{name: "dedupes repeats", content: "@lucy @lucy", want: []string{"lucy"}},
		{name: "multiple names", content: "@leader ask @designer", want: []string{"leader", "designer"}},
		{name: "hereby is not here", content: "@hereby noted", want: []string{"hereby"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMentionNames(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("parseMentionNames(%q) = %v, want %v", tt.content, got, tt.want)
			}
			for _, name := range tt.want {
				if !got[name] {
					t.Fatalf("parseMentionNames(%q) missing %q: %v", tt.content, name, got)
				}
			}
		})
	}
}

func TestHasHereName(t *testing.T) {
	tests := []struct {
		name string
		set  map[string]bool
		want bool
	}{
		{name: "exact lowercase", set: map[string]bool{"here": true}, want: true},
		{name: "uppercase", set: map[string]bool{"HERE": true}, want: true},
		{name: "mixed case", set: map[string]bool{"Here": true}, want: true},
		{name: "absent", set: map[string]bool{"lucy": true}, want: false},
		{name: "prefix does not match", set: map[string]bool{"hereby": true}, want: false},
		{name: "empty set", set: map[string]bool{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasHereName(tt.set); got != tt.want {
				t.Fatalf("hasHereName(%v) = %v, want %v", tt.set, got, tt.want)
			}
		})
	}
}

// mentionTestAgent seeds an agent and (optionally) adds it to the channel.
// Channel membership requires the agent's home channel to match (migration
// 000034 trigger), so member agents are homed to the channel under test.
func mentionTestAgent(t *testing.T, pool *pgxpool.Pool, ownerID, channelID, name string, isActive, isMember bool) string {
	t.Helper()
	id := uuid.NewString()
	var homeChannelID any
	if isMember {
		homeChannelID = channelID
	}
	_, err := pool.Exec(context.Background(),
		`INSERT INTO agents (id, name, owner_id, model_name, is_active, home_channel_id)
		 VALUES ($1, $2, $3, 'test-model', $4, $5)`,
		id, name, ownerID, isActive, homeChannelID,
	)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if isMember {
		_, err = pool.Exec(context.Background(),
			`INSERT INTO channel_members (channel_id, member_type, member_id) VALUES ($1, 'agent', $2)`,
			channelID, id,
		)
		if err != nil {
			t.Fatalf("add channel member: %v", err)
		}
	}
	return id
}

func TestResolveMentionsHereBroadcast(t *testing.T) {
	pool := agentRunTestPool(t)
	ctx := context.Background()
	ownerID := agentRunUser(t, pool)

	channelID := uuid.NewString()
	_, err := pool.Exec(ctx,
		`INSERT INTO channels (id, name, created_by) VALUES ($1, $2, $3)`,
		channelID, "mention-here-"+channelID[:8], ownerID,
	)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	activeA := mentionTestAgent(t, pool, ownerID, channelID, fmt.Sprintf("here-a-%s", channelID[:8]), true, true)
	activeB := mentionTestAgent(t, pool, ownerID, channelID, fmt.Sprintf("here-b-%s", channelID[:8]), true, true)
	// Inactive member and active non-member must not be notified.
	mentionTestAgent(t, pool, ownerID, channelID, fmt.Sprintf("here-off-%s", channelID[:8]), false, true)
	mentionTestAgent(t, pool, ownerID, channelID, fmt.Sprintf("here-out-%s", channelID[:8]), true, false)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM channels WHERE id = $1`, channelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, ownerID)
	})

	svc := NewMentionService(pool)

	assertIDs := func(t *testing.T, content string, want []string) {
		t.Helper()
		got, hasMentions, err := svc.ResolveMentions(ctx, content, channelID)
		if err != nil {
			t.Fatalf("ResolveMentions(%q): %v", content, err)
		}
		if !hasMentions {
			t.Fatalf("ResolveMentions(%q) hasMentions = false, want true", content)
		}
		if len(got) != len(want) {
			t.Fatalf("ResolveMentions(%q) = %v, want %v", content, got, want)
		}
		gotSet := make(map[string]bool, len(got))
		for _, id := range got {
			gotSet[id] = true
		}
		for _, id := range want {
			if !gotSet[id] {
				t.Fatalf("ResolveMentions(%q) missing %s: %v", content, id, got)
			}
		}
	}

	t.Run("here notifies all active channel agents", func(t *testing.T) {
		assertIDs(t, "@here standup time", []string{activeA, activeB})
	})

	t.Run("here is case insensitive", func(t *testing.T) {
		assertIDs(t, "@HERE sync now", []string{activeA, activeB})
	})

	t.Run("here plus explicit mention still broadcasts", func(t *testing.T) {
		assertIDs(t, "@here @nobody-", []string{activeA, activeB})
	})

	t.Run("no mentions unchanged", func(t *testing.T) {
		got, hasMentions, err := svc.ResolveMentions(ctx, "hello everyone", channelID)
		if err != nil {
			t.Fatalf("ResolveMentions: %v", err)
		}
		if hasMentions || got != nil {
			t.Fatalf("ResolveMentions(no mention) = %v, %v; want nil, false", got, hasMentions)
		}
	})

	t.Run("unknown mention resolves empty", func(t *testing.T) {
		assertIDs(t, fmt.Sprintf("@ghost-%s", channelID[:8]), []string{})
	})
}
