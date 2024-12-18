package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	log "github.com/sirupsen/logrus"
)

// max returns the maximum between two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum between two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Node represents a node in the tree (directory or file).
type Node struct {
	Name     string
	Children map[string]*Node
	IsFile   bool
}

// ArrayStringFlag are defined for string flags that may have multiple values.
type ArrayStringFlag []string

// Returns the concatenated string representation of the array of flags.
func (f *ArrayStringFlag) String() string {
	return fmt.Sprintf("%v", *f)
}

// Get returns an empty interface that may be type-asserted to the underlying
// value of type bool, string, etc.
func (f *ArrayStringFlag) Get() interface{} {
	return ""
}

// Set appends value the array of flags.
func (f *ArrayStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// removeDuplicates removes duplicates from a string array.
func removeDuplicates(strings []string) []string {
	unique := make(map[string]struct{})
	result := []string{}

	for _, str := range strings {
		if _, exists := unique[str]; !exists {
			unique[str] = struct{}{}
			result = append(result, str)
		}
	}

	return result
}

// sanitizePackageName cleans up the package name so that it is valid.
func sanitizePackageName(dirName string) string {
	partsDirName := strings.Split(dirName, "/")
	if len(partsDirName) > 1 {
		dirName = partsDirName[len(partsDirName)-2]
	}

	// Étape 1 : Remplacer les caractères non valides par _
	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return '_'
	}, dirName)

	// Étape 2 : Si le nom commence par un chiffre, ajouter un préfixe
	if len(sanitized) > 0 && unicode.IsDigit(rune(sanitized[0])) {
		sanitized = "_" + sanitized
	}

	// Étape 3 : S'assurer que le nom n'est pas vide
	if sanitized == "" || sanitized == "_" {
		sanitized = "main"
	}

	return sanitized
}

// findReposAndSubRepos finds all repositories and sub-repositories in the current directory.
func (j *job) findReposAndSubRepos() error {
	currentPath := j.fileDir

	log.Infof("currentPath:\n%s", currentPath)

	var basePath string

	// Remonter jusqu'à trouver go.mod, main.go ou atteindre la racine
	for {
		// Chemins potentiels pour go.mod et main.go
		goModPath := filepath.Join(currentPath, "go.mod")
		mainGoPath := filepath.Join(currentPath, "main.go")

		// Vérifier si go.mod ou main.go existe dans le répertoire courant
		if _, err := os.Stat(goModPath); err == nil {
			basePath = currentPath
			break
		}
		if _, err := os.Stat(mainGoPath); err == nil {
			basePath = currentPath
			break
		}

		// Si on atteint la racine du système, on s'arrête
		if currentPath == filepath.Dir(currentPath) {
			basePath = currentPath
			break
		}

		// Remonter d'un niveau
		currentPath = filepath.Dir(currentPath)
	}

	log.Infof("currentPath v2:\n%s", currentPath)

	// Parcourir tous les sous-répertoires à partir de basePath
	var repos []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			repos = append(repos, path)
		}
		// Ajouter main.go s'il est trouvé
		if !info.IsDir() && info.Name() == "main.go" {
			repos = append(repos, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking the path %s: %w", basePath, err)
	}

	log.Infof("currentPath v3:\n%s", repos)
	j.repoStructure = formatRepo(repos)
	log.Infof("Found repositories and sub-repositories:\n%s", j.repoStructure)
	return nil
}

// AddPath adds a path to the tree.
func (n *Node) AddPath(pathParts []string) {
	if len(pathParts) == 0 {
		return
	}

	childName := pathParts[0]
	child, exists := n.Children[childName]
	if !exists {
		child = &Node{
			Name:     childName,
			Children: make(map[string]*Node),
		}
		n.Children[childName] = child
	}

	if len(pathParts) == 1 {
		child.IsFile = true
	} else {
		child.AddPath(pathParts[1:])
	}
}

// Format returns a string representation of the tree.
func (n *Node) Format(indent string, skipRoot bool) string {
	var builder strings.Builder

	// Ne pas afficher la racine si `skipRoot` est vrai
	if !skipRoot {
		if n.IsFile {
			builder.WriteString(fmt.Sprintf("%s  - %s\n", indent, n.Name))
		} else {
			builder.WriteString(fmt.Sprintf("%s- %s/\n", indent, n.Name))
		}
	}

	// Trier les enfants par ordre alphabétique pour un affichage stable
	var childKeys []string
	for key := range n.Children {
		childKeys = append(childKeys, key)
	}
	sort.Strings(childKeys)

	// Ajouter les enfants au résultat
	for _, key := range childKeys {
		builder.WriteString(n.Children[key].Format(indent+"  ", false))
	}

	return builder.String()
}

// formatRepo formats the given paths into a tree.
func formatRepo(paths []string) string {
	root := &Node{
		Name:     "toto", // Nom de la racine
		Children: make(map[string]*Node),
	}

	// Construire l'arbre à partir des chemins donnés
	for _, path := range paths {
		normalizedPath := filepath.Clean(path)
		pathParts := strings.Split(normalizedPath, string(filepath.Separator))
		if pathParts[0] == "." {
			pathParts = pathParts[1:]
		}
		root.AddPath(pathParts)
	}

	// Retourner l'arborescence formatée sans imprimer la racine explicitement
	return root.Format("", true)
}
