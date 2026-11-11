package maxbot

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

const keyboardAttachment = `{"type":"inline_keyboard","payload":{"buttons":[[{"type":"link","text":"Read","url":"https://max.ru/join/example"},{"type":"callback","text":"Back","payload":"planning-back"}]]}}`
const shareAttachment = `{"type":"share","payload":{"url":"https://max.ru/join/example","token":"private-preview-token"},"title":"Preview"}`

func TestWebhookAcceptsKeyboardAndForwardedPreview(t *testing.T) {
	api, err := New("unused-test-token")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"message_created", "message_edited"} {
		for _, body := range []string{"null", "[]", "[" + shareAttachment + "]", "[" + keyboardAttachment + "]"} {
			t.Run(kind+"/"+body, func(t *testing.T) {
				fixture := `{"update_type":"` + kind + `","timestamp":1794401956491,"message":{"sender":{"user_id":101},"recipient":{"chat_type":"dialog","chat_id":202,"user_id":303},"body":{"mid":"mid.incoming","text":"","attachments":` + body + `},"link":{"type":"forward","message":{"mid":"mid.source","text":"Read - https://max.ru/join/example","attachments":[` + shareAttachment + `,` + keyboardAttachment + `]}}}}`
				updates := make(chan schemes.UpdateInterface, 1)
				response := httptest.NewRecorder()
				api.GetHandler(updates)(response, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(fixture)))
				if response.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
				}
				var message schemes.Message
				switch update := (<-updates).(type) {
				case *schemes.MessageCreatedUpdate:
					message = update.Message
				case *schemes.MessageEditedUpdate:
					message = update.Message
				default:
					t.Fatalf("unexpected update %T", update)
				}
				if message.Link.Message.Text != "Read - https://max.ru/join/example" || len(message.Link.Message.Attachments) != 2 {
					t.Fatalf("forwarded message lost: %#v", message.Link)
				}
				keyboard := message.Link.Message.Attachments[1].(*schemes.InlineKeyboardAttachment)
				link := keyboard.Payload.Buttons[0][0].GetUrl()
				if link == nil || *link != "https://max.ru/join/example" {
					t.Fatal("link URL lost")
				}
				if body != "[]" && body != "null" && len(message.Body.Attachments) != 1 {
					t.Fatal("body attachments lost")
				}
			})
		}
	}
}

func TestWebhookAcceptsBackCallbackWithPreview(t *testing.T) {
	api, _ := New("unused-test-token")
	updates := make(chan schemes.UpdateInterface, 1)
	fixture := `{"update_type":"message_callback","timestamp":1794401956491,"callback":{"callback_id":"opaque","payload":"planning-back","user":{"user_id":101}},"message":{"body":{"mid":"mid.prompt","text":"Instructions","attachments":[` + keyboardAttachment + `,` + shareAttachment + `]}}}`
	response := httptest.NewRecorder()
	api.GetHandler(updates)(response, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(fixture)))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	update := (<-updates).(*schemes.MessageCallbackUpdate)
	if update.Callback.Payload != "planning-back" || update.GetUserID() != 101 {
		t.Fatalf("callback changed: %#v", update.Callback)
	}
}

func TestWebhookLogsDecodeReasonWithoutMessageContents(t *testing.T) {
	api, _ := New("unused-test-token")
	var logs bytes.Buffer
	old := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(old) })
	fixture := `{"update_type":"message_created","timestamp":1794401956491,"message":{"sender":{"user_id":101},"body":{"mid":"mid.bad","seq":"invalid","text":"PRIVATE TEXT","attachments":[` + shareAttachment + `]}}}`
	response := httptest.NewRecorder()
	updates := make(chan schemes.UpdateInterface, 1)
	api.GetHandler(updates)(response, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(fixture)))
	if response.Code != http.StatusBadRequest || len(updates) != 0 {
		t.Fatal("malformed update was accepted")
	}
	for _, want := range []string{`update_type="message_created"`, "user_id=101", `message_id="mid.bad"`, "cannot unmarshal string", "seq"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("missing %q in %s", want, logs.String())
		}
	}
	for _, secret := range []string{"PRIVATE TEXT", "private-preview-token", "unused-test-token"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("log leaked %q", secret)
		}
	}
}
