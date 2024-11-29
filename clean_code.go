package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

// commentUnusedFunctions ajoute des commentaires aux fonctions non utilisées.
func (j *job) commentUnusedFunctions(funcs []string) error {
	// Lire le fichier ligne par ligne
	content, err := os.ReadFile(j.fileDir + "/" + j.currentFileName)
	if err != nil {
		return err
	}

	// Transformation du contenu en lignes pour faciliter la manipulation
	lines := bytes.Split(content, []byte("\n"))

	// On crée une liste de lignes modifiées pour reconstruire le fichier avec les commentaires ajoutés
	var updatedLines [][]byte
	commentMode := false
	openBraces := 0

	for _, line := range lines {
		lineStr := string(line)

		// Si on est en mode commentaire, on ajoute `//` au début de la ligne
		if commentMode {
			updatedLines = append(updatedLines, append([]byte("// "), line...))

			// Compter les accolades ouvrantes et fermantes pour détecter la fin de la fonction
			openBraces += bytes.Count(line, []byte("{"))
			openBraces -= bytes.Count(line, []byte("}"))

			// Fin du bloc de fonction
			if openBraces == 0 {
				commentMode = false
			}
			continue
		}

		// Détection du début de la fonction non utilisée
		for _, fn := range funcs {
			if match, _ := regexp.MatchString(fmt.Sprintf(`^func %s\(`, fn), lineStr); match {
				commentMode = true
				openBraces = 1 // Initialiser le compte d'accolades pour cette fonction
				updatedLines = append(updatedLines, append([]byte("// "), line...))
				break
			}
		}

		// Si on n'est pas en mode commentaire, ajouter la ligne telle quelle
		if !commentMode {
			updatedLines = append(updatedLines, line)
		}
	}

	// Reconstruire le fichier avec les lignes mises à jour
	return os.WriteFile(j.fileDir+"/"+j.currentFileName, bytes.Join(updatedLines, []byte("\n")), 0644)
}

// fixImports exécute goimports pour corriger les importations dans le fichier.
func (j *job) fixImports() error {
	// Commande pour exécuter goimports
	cmd := exec.Command("goimports", "-w", j.currentFileName)
	cmd.Dir = j.fileDir

	// Exécution de la commande
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(j.t("error running goimports")+": %v - %s", err, out.String())
	}

	return nil
}

// findUnusedFunctions utilise staticcheck pour trouver les fonctions non utilisées dans le fichier.
func (j *job) findUnusedFunctions() ([]string, error) {
	fmt.Println(fmt.Sprintf(j.t("Analysis of")+" : %s", j.fileDir+"/"+j.currentFileName))

	// Construire la commande staticcheck avec le chemin complet
	cmd := exec.Command("staticcheck", j.currentFileName)

	// Spécifier le répertoire de travail pour staticcheck
	cmd.Dir = j.fileDir

	var out bytes.Buffer
	cmd.Stderr = &out
	cmd.Stdout = &out

	// Exécuter la commande et capturer les avertissements
	err := cmd.Run()
	output := out.String()

	// Si une erreur de statut de sortie est retournée, vérifier si la sortie contient des avertissements U1000
	if err != nil {
		fmt.Println(j.t("Warning or error")+":", err) // Affiche l'erreur pour diagnostic
	}

	// Regex pour détecter les fonctions non utilisées signalées par U1000
	re := regexp.MustCompile(`func (\w+) is unused`)
	matches := re.FindAllStringSubmatch(output, -1)

	var unusedFuncs []string
	for _, match := range matches {
		if len(match) > 1 {
			unusedFuncs = append(unusedFuncs, match[1])
		}
	}

	// Retourner les fonctions inutilisées même si staticcheck a généré une erreur de statut
	return unusedFuncs, nil // Ignorer l'erreur pour continuer normalement
}
