package utils

import (
	"encoding/json"
	"os"
)

type ImageConfig struct {
	Disks         []Disk         `json:"Disks"`
	SystemConfigs []SystemConfig `json:"SystemConfigs"`
}

// N.B. Partial definition -- just enough for what we need.
type Disk struct {
	Artifacts []Artifact `json:"Artifacts"`
}

type Artifact struct {
	Compression string `json:"Compression"`
	Name        string `json:"Name"`
	Type        string `json:"Type"`
}

// N.B. Partial definition -- just enough for what we need.
type SystemConfig struct {
	BootType string `json:"BootType"`
	Name     string `json:"Name"`
}

func ParseImageConfig(configFilePath string) (*ImageConfig, error) {
	bytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	var config ImageConfig
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
