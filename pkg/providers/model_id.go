package providers

import "strings"

// ParseModelID separates "z-ai/glm5@nvidia" into ("z-ai/glm5", "nvidia").
// If no @ is present, returns (raw, "").
func ParseModelID(raw string) (model, provider string) {
	idx := strings.LastIndex(raw, "@")
	if idx < 0 {
		return raw, ""
	}
	return raw[:idx], raw[idx+1:]
}

// FormatModelID combines model + provider into "model@provider".
// If provider is empty, returns just the model.
func FormatModelID(model, provider string) string {
	if provider == "" {
		return model
	}
	return model + "@" + provider
}
