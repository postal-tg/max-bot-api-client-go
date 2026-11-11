package schemes

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKeyboardUnmarshalButtonTypes(t *testing.T) {
	data := `{"buttons":[[
		{"type":"link","text":"Read","url":"https://example.com/"},
		{"type":"callback","text":"Back","payload":"planning-back","intent":"default"},
		{"type":"request_contact","text":"Contact"},
		{"type":"request_geo_location","text":"Location","quick":true},
		{"type":"open_app","text":"App","web_app":"example_bot","payload":"start","contact_id":42}
	],[{"type":"clipboard","text":"Copy","payload":"keep this"},{"type":"message","text":"Send"}]]}`
	var keyboard Keyboard
	if err := json.Unmarshal([]byte(data), &keyboard); err != nil {
		t.Fatal(err)
	}
	if len(keyboard.Buttons) != 2 || len(keyboard.Buttons[0]) != 5 {
		t.Fatalf("unexpected rows: %#v", keyboard.Buttons)
	}
	link := keyboard.Buttons[0][0]
	if link.GetType() != LINK || link.GetText() != "Read" || link.GetUrl() == nil || *link.GetUrl() != "https://example.com/" {
		t.Fatalf("link data lost: %#v", link)
	}
	callback := keyboard.Buttons[0][1].(*CallbackButton)
	if callback.Payload != "planning-back" || callback.Intent != DEFAULT {
		t.Fatalf("callback data lost: %#v", callback)
	}
	if _, ok := keyboard.Buttons[0][2].(*RequestContactButton); !ok {
		t.Fatal("contact type lost")
	}
	if !keyboard.Buttons[0][3].(*RequestGeoLocationButton).Quick {
		t.Fatal("location flag lost")
	}
	app := keyboard.Buttons[0][4].(*OpenAppButton)
	if app.WebApp != "example_bot" || app.Payload != "start" || app.ContactId != 42 {
		t.Fatalf("app data lost: %#v", app)
	}
	encoded, err := json.Marshal(keyboard.Buttons[1][0])
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"type":"clipboard","text":"Copy","payload":"keep this"}` {
		t.Fatalf("unknown button changed: %s", encoded)
	}
	if keyboard.Buttons[1][1].GetType() != MESSAGE {
		t.Fatal("message type lost")
	}
}

func TestKeyboardUnmarshalRejectsMalformedButtonWithoutReplacingState(t *testing.T) {
	keyboard := Keyboard{Buttons: [][]ButtonInterface{{Button{Type: LINK, Text: "keep"}}}}
	err := json.Unmarshal([]byte(`{"buttons":[[{"type":"link","text":123}]]}`), &keyboard)
	if err == nil || !strings.Contains(err.Error(), "button[0][0]") {
		t.Fatalf("expected indexed error, got %v", err)
	}
	if keyboard.Buttons[0][0].GetText() != "keep" {
		t.Fatal("failed decode replaced the keyboard")
	}
}

func TestKeyboardUnmarshalEmptyRows(t *testing.T) {
	for _, value := range []string{`{}`, `{"buttons":null}`, `{"buttons":[]}`, `{"buttons":[null,[]]}`} {
		var keyboard Keyboard
		if err := json.Unmarshal([]byte(value), &keyboard); err != nil {
			t.Fatalf("%s: %v", value, err)
		}
	}
}
