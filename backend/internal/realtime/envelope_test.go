package realtime

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestEnvelope_JSONRoundTrip(t *testing.T) {
	env := Envelope{
		Topic:   "ops.health",
		Type:    MessageTypeUpdate,
		Payload: json.RawMessage(`{"status":"ok"}`),
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Envelope
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Topic != env.Topic || got.Type != env.Type || string(got.Payload) != string(env.Payload) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, env)
	}
}

func TestEnvelope_Validate(t *testing.T) {
	tests := []struct {
		name    string
		env     Envelope
		wantErr bool
	}{
		{"valid update", Envelope{Topic: "ops.health", Type: MessageTypeUpdate}, false},
		{"valid subscribe control", Envelope{Topic: "ops.health", Type: MessageTypeSubscribe}, false},
		{"missing topic", Envelope{Type: MessageTypeUpdate}, true},
		{"missing type", Envelope{Topic: "ops.health"}, true},
		{"unknown type", Envelope{Topic: "ops.health", Type: MessageType("bogus")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.env.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthenticatorFunc_SatisfiesInterface(t *testing.T) {
	var a Authenticator = AuthenticatorFunc(func(r *http.Request) (Identity, error) {
		return Identity{UserID: 1, Role: "user"}, nil
	})
	got, err := a.Authenticate(nil)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got.UserID != 1 || got.Role != "user" {
		t.Fatalf("got %+v", got)
	}
}
