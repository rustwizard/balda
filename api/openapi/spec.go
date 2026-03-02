package openapi

import _ "embed"

//go:embed http-api.yaml
var Spec []byte
