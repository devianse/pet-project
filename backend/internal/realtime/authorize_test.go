package realtime

import (
	"context"
	"testing"
)

func TestTopicAuthorizerFunc_SatisfiesInterface(t *testing.T) {
	var a TopicAuthorizer = TopicAuthorizerFunc(func(_ context.Context, id Identity, topic string) bool {
		return id.Role == "admin" && topic == "ops.health"
	})
	if !a.Authorize(context.Background(), Identity{Role: "admin"}, "ops.health") {
		t.Fatal("expected admin to be authorized for ops.health")
	}
	if a.Authorize(context.Background(), Identity{Role: "user"}, "ops.health") {
		t.Fatal("expected non-admin to be rejected for ops.health")
	}
}

func TestAllowAuthenticated_PermitsAnyTopic(t *testing.T) {
	if !AllowAuthenticated.Authorize(context.Background(), Identity{UserID: 1, Role: "user"}, "anything.at.all") {
		t.Fatal("expected AllowAuthenticated to permit any topic")
	}
}
