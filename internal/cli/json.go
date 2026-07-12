package cli

import (
	"encoding/json"
)

// jsonMarshal wraps encoding/json.Marshal for use in cli package
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
