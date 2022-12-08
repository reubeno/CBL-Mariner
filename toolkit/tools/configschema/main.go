package main

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"

	"github.com/invopop/jsonschema"

	"github.com/reubeno/CBL-Mariner/toolkit/tools/imagegen/configuration"
)

func main() {
	var reflector jsonschema.Reflector
	reflector.RequiredFromJSONSchemaTags = true
	reflector.AdditionalFields = additionalFieldsHandler
	schema := reflector.Reflect(&configuration.Config{})
	content, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal schema to JSON")
	}

	fmt.Printf("%s\n", content)
}

// Add $schema
func additionalFieldsHandler(ty reflect.Type) []reflect.StructField {
	var fields []reflect.StructField
	if ty.Name() == "Config" {
		fields = append(fields, reflect.StructField{
			Name:    "Schema",
			PkgPath: "",
			Tag:     `json:"$schema"`,
			Type:    reflect.TypeOf(""),
		})
	}
	return fields
}
