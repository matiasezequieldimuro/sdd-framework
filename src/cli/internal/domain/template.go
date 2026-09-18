package domain

import (
	"strings"
)

// RenderTemplate reemplaza variables {{var}} en una cadena
func RenderTemplate(content string, vars map[string]string) string {
	result := content
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
