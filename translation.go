package main

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed i18n/*.json
var translations embed.FS

type Translations map[string]string

func (j *job) loadTranslations() error {
	var translationFile string
	switch j.lang {
	case "fr":
		translationFile = "i18n/fr.json"
	case "en":
		translationFile = "i18n/en.json"
	default:
		return fmt.Errorf("unsupported language: %v", j.lang)
	}

	file, err := translations.ReadFile(translationFile)
	if err != nil {
		return fmt.Errorf("could not read translation file: %v", err)
	}

	var translations Translations
	if err := json.Unmarshal(file, &translations); err != nil {
		return fmt.Errorf("could not unmarshal translation file: %v", err)
	}

	j.trad = translations

	return nil
}

// Fonction pour obtenir une traduction
func (j *job) t(key string) string {
	if val, ok := j.trad[key]; ok {
		return val
	}
	return key
}
