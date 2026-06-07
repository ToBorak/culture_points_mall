package llm

import (
	"context"
	"testing"
)

type fakeClient struct{ reply string }

func (f fakeClient) Messages(_ context.Context, _ MessagesRequest) (MessagesResponse, error) {
	return MessagesResponse{Content: []Block{{Type: "text", Text: f.reply}}}, nil
}
func (f fakeClient) MessagesStream(_ context.Context, _ MessagesRequest) (<-chan StreamEvent, error) {
	return nil, nil
}

func TestMessagesJSON_StripsCodeFence(t *testing.T) {
	c := fakeClient{reply: "```json\n{\"a\":1}\n```"}
	got, err := MessagesJSON(context.Background(), c, "sys", "u", 0)
	if err != nil || got != `{"a":1}` {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestMessagesJSON_ExtractsBraces(t *testing.T) {
	c := fakeClient{reply: "废话{\"a\":1}尾巴"}
	got, _ := MessagesJSON(context.Background(), c, "sys", "u", 0)
	if got != `{"a":1}` {
		t.Fatalf("got %q", got)
	}
}

func TestMessagesText_Concat(t *testing.T) {
	c := fakeClient{reply: "  hello  "}
	got, _ := MessagesText(context.Background(), c, "sys", "u", 0)
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}
