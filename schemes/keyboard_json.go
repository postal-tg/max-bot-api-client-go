package schemes

import (
	"encoding/json"
	"fmt"
)

// UnmarshalJSON selects a concrete type for each ButtonInterface value.
func (k *Keyboard) UnmarshalJSON(data []byte) error {
	var raw struct {
		Buttons [][]json.RawMessage `json:"buttons"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var rows [][]ButtonInterface
	if raw.Buttons != nil {
		rows = make([][]ButtonInterface, len(raw.Buttons))
	}
	for i, row := range raw.Buttons {
		if row != nil {
			rows[i] = make([]ButtonInterface, len(row))
		}
		for j, value := range row {
			var base Button
			if err := json.Unmarshal(value, &base); err != nil {
				return fmt.Errorf("button[%d][%d]: %w", i, j, err)
			}
			var button ButtonInterface
			switch base.Type {
			case LINK:
				button = new(LinkButton)
			case CALLBACK:
				button = new(CallbackButton)
			case CONTACT:
				button = new(RequestContactButton)
			case GEOLOCATION:
				button = new(RequestGeoLocationButton)
			case OPEN_APP:
				button = new(OpenAppButton)
			default:
				// Preserve fields from button types introduced by newer MAX clients.
				rows[i][j] = &rawButton{Button: base, data: append(json.RawMessage(nil), value...)}
				continue
			}
			if err := json.Unmarshal(value, button); err != nil {
				return fmt.Errorf("button[%d][%d]: %w", i, j, err)
			}
			rows[i][j] = button
		}
	}
	k.Buttons = rows
	return nil
}

func (b LinkButton) GetUrl() *string {
	return &b.Url
}

type rawButton struct {
	Button
	data json.RawMessage
}

func (b rawButton) MarshalJSON() ([]byte, error) {
	return b.data, nil
}
