package bot

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseChatID(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int64
		// wantErr is true when the value cannot be converted at all; the caller turns
		// that into a request error.
		wantErr bool
		// wantUnsupported is true when the type itself is unknown; the caller logs a
		// warning and sends the request without rate limiting, so it must stay
		// distinguishable from a conversion failure.
		wantUnsupported bool
	}{
		{
			name:  "string, the shape gotgbot sends",
			value: "-1001234567890",
			want:  -1001234567890,
		},
		{
			name:  "int64",
			value: int64(-1001234567890),
			want:  -1001234567890,
		},
		{
			name:  "int",
			value: 123456,
			want:  123456,
		},
		{
			name:  "int32",
			value: int32(123456),
			want:  123456,
		},
		{
			name:  "json.Number",
			value: json.Number("-1001234567890"),
			want:  -1001234567890,
		},
		{
			// A chat referenced by @username has no ID to key a limiter on.
			name:    "unparsable string",
			value:   "@my_channel",
			wantErr: true,
		},
		{
			name:            "unexpected type",
			value:           []string{"123"},
			wantErr:         true,
			wantUnsupported: true,
		},
		{
			// Floats are rejected rather than truncated: a chat ID that lost precision
			// would key the wrong limiter.
			name:            "float",
			value:           1.0,
			wantErr:         true,
			wantUnsupported: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChatID(tt.value)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseChatID(%v) = %d, want an error", tt.value, got)
				}
				if unsupported := errors.Is(err, errUnsupportedChatIDType); unsupported != tt.wantUnsupported {
					t.Fatalf(
						"parseChatID(%v) errors.Is(errUnsupportedChatIDType) = %v, want %v (err: %v)",
						tt.value, unsupported, tt.wantUnsupported, err,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseChatID(%v) unexpected error: %v", tt.value, err)
			}
			if got != tt.want {
				t.Errorf("parseChatID(%v) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
