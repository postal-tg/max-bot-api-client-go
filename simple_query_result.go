package maxbot

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

// decodeSimpleQueryResult reads the full body, unmarshals it into SimpleQueryResult and returns raw bytes too.
// This is needed because MAX may put additional fields into the JSON that are useful for debugging.
func decodeSimpleQueryResult(body io.ReadCloser) (*schemes.SimpleQueryResult, []byte, error) {
	defer func() { _ = body.Close() }()
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, err
	}

	res := new(schemes.SimpleQueryResult)
	if err := json.Unmarshal(raw, res); err != nil {
		return nil, raw, err
	}

	return res, raw, nil
}

func newSimpleQueryAPIError(op string, res *schemes.SimpleQueryResult, raw []byte) error {
	if res == nil || res.Success {
		return nil
	}

	return &APIError{
		Code:       http.StatusOK,
		HTTPStatus: "OK",
		Message:    strings.TrimSpace(res.Message),
		Details:    op,
		RawBody:    string(raw),
	}
}
