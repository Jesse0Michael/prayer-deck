package main

// ClientConfig is the runtime configuration the frontend needs before it can
// reach the API.
type ClientConfig struct {
	APIURL string `json:"apiURL"`
}

// ConfigService exposes ClientConfig to the frontend, so a deployed build can
// point at a different API without rebuilding the frontend.
type ConfigService struct {
	config ClientConfig
}

// Config returns the client configuration.
func (s *ConfigService) Config() ClientConfig {
	return s.config
}
