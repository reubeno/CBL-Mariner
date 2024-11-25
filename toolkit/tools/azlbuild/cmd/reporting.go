package cmd

import (
	"encoding/json"
	"fmt"
)

func ReportResult(result interface{}) error {
	jsonData, err := json.Marshal(result)
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))
	return nil
}
