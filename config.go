package main

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	yaml "gopkg.in/yaml.v3"

	"github.com/ariden/goia/secret"
)

type fileSource uint8

const (
	fileSourceStdin fileSource = iota
	fileSourceFilePath
)

type Config struct {
	// Local is the root Go module name. All subpackages of this module
	// will be separated from the external packages.
	Local string `yaml:"local"`

	// Prefixes is a list of relative Go packages from the root package.
	// All comments with these prefixes will be separated from each other.
	Prefixes []string `yaml:"prefixes"`

	MaxAttempts       int     `yaml:"max_attempts"`
	Lang              string  `yaml:"language"`
	OpenAIKey         string  `yaml:"openai_api_key"`
	OpenAIMaxTokens   int     `yaml:"openai_max_tokens"`
	OpenAIModel       string  `yaml:"openai_model"`
	OpenAITemperature float64 `yaml:"openai_temperature"`
	OpenAIURL         string  `yaml:"openai_url"`
}

// ConfigCache is a cache to contains the configuration for all processed files.
type ConfigCache struct {
	rootConfig Config
	configs    map[string]*Config
}

// NewConfigCache instantiates a new cache to store the configuration for all processed files.
func NewConfigCache(local string, prefixes []string) *ConfigCache {
	return &ConfigCache{
		rootConfig: Config{
			Local:    local,
			Prefixes: prefixes,
		},
		configs: make(map[string]*Config),
	}
}

func (j *job) updateCache() error {
	dirPath := j.fileDir + "/" + j.fileName
	if j.source == fileSourceStdin {
		dirPath = j.fileDir + "/..."
	}

	cfg, err := j.cache.Get(dirPath)
	if err != nil {
		return err
	}

	j.openAITemperature = cfg.OpenAITemperature
	j.openAIURL = cfg.OpenAIURL
	j.openAIModel = cfg.OpenAIModel
	if cfg.Lang != "" {
		j.lang = cfg.Lang
	}
	j.openAIApiKey = secret.String(cfg.OpenAIKey)
	j.openAIMaxTokens = cfg.OpenAIMaxTokens
	j.maxAttempts = cfg.MaxAttempts

	if err := j.loadTranslations(); err != nil {
		return err
	}
	return nil
}

// Merge merges the given Config with this configure and return
// a new Config with the merged result.
// - Local attribute value is overriden.
// - Prefixes attribute values are appended.
func (cfg *Config) Merge(newCfg *Config) *Config {
	if newCfg.Local != "" {
		cfg.Local = newCfg.Local
	}

	{
		if newCfg.OpenAIURL != "" {
			cfg.OpenAIURL = newCfg.OpenAIURL
		}
		if newCfg.Lang != "" {
			cfg.Lang = newCfg.Lang
		}
		if newCfg.OpenAIKey != "" {
			cfg.OpenAIKey = newCfg.OpenAIKey
		}
		if newCfg.OpenAIModel != "" {
			cfg.OpenAIModel = newCfg.OpenAIModel
		}
		if newCfg.OpenAITemperature != 0.0 {
			cfg.OpenAITemperature = newCfg.OpenAITemperature
		}
		if newCfg.OpenAIMaxTokens != 0 {
			cfg.OpenAIMaxTokens = newCfg.OpenAIMaxTokens
		}
		if newCfg.MaxAttempts != 0 {
			cfg.MaxAttempts = newCfg.MaxAttempts
		}
	}

	return cfg
}

// Get returns the configuration for the given processed file.
// Keep all intermediate configurations in the cache.
func (cache *ConfigCache) Get(filename string) (*Config, error) {
	absFilepath, _ := filepath.Abs(filename)
	dirPath := filepath.Dir(absFilepath)

	cfg, err := cache.get(dirPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cache *ConfigCache) get(dirPath string) (*Config, error) {
	cfg, ok := cache.configs[dirPath]
	if ok {
		if cfg == nil {
			panic("cfg should not be nil")
		}
		return cfg, nil
	}

	var parentCfg *Config
	goModFilepath := filepath.Join(dirPath, "go.mod")
	modname, goModFileExists, err := getModuleNameFromGoModFile(goModFilepath)
	if err != nil {
		return nil, err
	}

	if !goModFileExists && dirPath != "." && dirPath != "/" {
		parentCfg, err = cache.get(filepath.Dir(dirPath))
		if err != nil {
			return nil, err
		}
	} else {
		parentCfg = &cache.rootConfig
		if parentCfg.Local == "" {
			parentCfg.Local = modname
		}
	}

	// Nouvelle étape : Vérifier le fichier .goia dans le répertoire HOME si nécessaire
	var homeCfg *Config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	homeCfgPath := filepath.Join(homeDir, ".goia")
	homeCfg, err = readConfigFile(homeCfgPath)
	if err != nil && !os.IsNotExist(err) { // Ignorer si le fichier n'existe pas
		return nil, err
	}

	localCfg, err := readConfigFile(filepath.Join(dirPath, ".goia"))
	if err != nil {
		return nil, err
	}

	// Fusionner les configurations
	cfg = parentCfg
	if homeCfg != nil {
		cfg = cfg.Merge(homeCfg) // Fusionner la config HOME si trouvée
	}
	if localCfg != nil {
		cfg = cfg.Merge(localCfg) // Fusionner la config locale si trouvée
	}

	if cfg == nil {
		panic("cfg should not be nil")
	}

	cache.configs[dirPath] = cfg
	return cfg, nil
}

func readConfigFile(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	defer func() {
		_ = f.Close()
	}()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

var goModModuleRegexp = regexp.MustCompile(`^module\s+(\S+)$`)

func getModuleNameFromGoModFile(goModFilepath string) (string, bool, error) {
	var modname string

	f, err := os.Open(goModFilepath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}

		return "", false, err
	}

	defer func() {
		_ = f.Close()
	}()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if m := goModModuleRegexp.FindStringSubmatch(line); m != nil {
			modname = m[1]
			break
		}
	}

	if err := s.Err(); err != nil {
		return "", true, err
	}

	return modname, true, nil
}
