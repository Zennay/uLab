package evidence

import (
	"encoding/json"
	"os"

	"github.com/Zennay/ulab/internal/engine"
)

func WriteJSON(path string, result engine.RunResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '
')
	return os.WriteFile(path, data, 0o644)
}
