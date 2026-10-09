package ocr

import (
	"fmt"
	"strings"
)

// DefaultPrompt returns the default OCR system prompt with language placeholder.
const defaultPromptTemplate = `You are an OCR engine. The image contains subtitle text in %s.
Return ONLY the exact text visible in the image, preserving line breaks.
Do not add explanations, translations, or formatting.`

// BuildPrompt returns the system prompt with the language filled in.
// If customPrompt is non-empty, it's used instead (with {language} replaced).
func BuildPrompt(language, customPrompt string) string {
	if customPrompt != "" {
		return strings.ReplaceAll(customPrompt, "{language}", language)
	}
	return fmt.Sprintf(defaultPromptTemplate, language)
}

