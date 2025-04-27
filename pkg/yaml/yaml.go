package yaml

import (
	"bytes"
	"strings"

	"sigs.k8s.io/yaml"
)

// processTemplateStrings ensures template strings are properly quoted and not broken into multiple lines
func processTemplateStrings(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		if strings.Contains(line, "{{") && strings.Contains(line, "}}") {
			// Quote the entire line if it contains a template
			line = strings.ReplaceAll(line, "\"", "\\\"")
			line = strings.ReplaceAll(line, "'", "\"")
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// Indent - adds indentation to given content.
func Indent(content []byte, n int) []byte {
	if n < 0 {
		return content
	}
	prefix := append([]byte("\n"), bytes.Repeat([]byte(" "), n)...)
	content = append(prefix[1:], content...)
	return bytes.ReplaceAll(content, []byte("\n"), prefix)
}

// Marshal object to yaml string with indentation.
func Marshal(object interface{}, indent int) (string, error) {
	objectBytes, err := yaml.Marshal(object)
	if err != nil {
		return "", err
	}
	content := processTemplateStrings(string(objectBytes))
	objectBytes = []byte(content)
	objectBytes = Indent(objectBytes, indent)
	objectBytes = bytes.TrimRight(objectBytes, "\n ")
	return string(objectBytes), nil
}
