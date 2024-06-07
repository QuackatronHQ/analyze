package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/DeepSourceCorp/artifacts/types"
	"github.com/bmatcuk/doublestar/v4"
)

var code = flag.String("code", "./", "code path")
var language = flag.String("language", "", "language")

func main() {
	flag.Parse()
	path, err := filepath.Abs(*code)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	// name := filepath.Base(path)

	files, err := Files(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	dsConfig, err := ParseDSConfig(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	config, err := GenerateConfig(path, files, dsConfig)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	err = Write(path, config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
}

func GenerateConfig(dir string, files []string, dsConfig *types.DSConfig) (*types.AnalysisConfig, error) {
	files = StripPathPrefix(dir, files)
	for i, f := range files {
		files[i] = filepath.Join("/code", f)
	}

	analysisConfig := &types.AnalysisConfig{
		ExcludePatterns: []string{},
		TestFiles:       []string{},
		TestPatterns:    []string{},
	}

	if dsConfig != nil {
		files, excludedFiles, err := FilterFiles("/code", files, dsConfig.ExcludePatterns)
		if err != nil {
			return nil, err
		}
		analysisConfig.ExcludePatterns = dsConfig.ExcludePatterns
		analysisConfig.ExcludeFiles = excludedFiles
		analysisConfig.Files = files

		_, excludedFiles, err = FilterFiles("/code", files, dsConfig.TestPatterns)
		if err != nil {
			return nil, err
		}

		analysisConfig.TestPatterns = dsConfig.TestPatterns
		analysisConfig.TestFiles = excludedFiles

		for _, v := range dsConfig.Analyzers {
			if v.Name == *language {
				analysisConfig.AnalyzerMeta = v
				break
			}
		}
	} else {
		analysisConfig.Files = files
		switch *language {
		case "java":
			analysisConfig.AnalyzerMeta = map[string]interface{}{
				"meta": map[string]interface{}{
					"java_version": 17,
				},
			}
		default:
			analysisConfig.AnalyzerMeta = map[string]interface{}{}
		}
	}

	return analysisConfig, nil

}

func Files(path string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(path, func(path string, d os.DirEntry, walkErr error) error {
		fi, statErr := os.Lstat(path)
		if statErr != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func StripPathPrefix(prefix string, files []string) []string {
	stripped := []string{}
	for _, file := range files {
		stripped = append(stripped, file[len(prefix):])
	}

	return stripped
}

func HasTOML(dir string) bool {
	path := filepath.Join(dir, ".deepsource.toml")
	_, err := os.Stat(path)
	return err == nil
}

func ParseDSConfig(dir string) (*types.DSConfig, error) {
	if !HasTOML(dir) {
		return nil, nil
	}

	path := filepath.Join(dir, ".deepsource.toml")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	config := &types.DSConfig{}

	_, err = toml.NewDecoder(file).Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func FilterFiles(prefix string, files []string, patterns []string) ([]string, []string, error) {
	filtered := []string{}
	excluded := []string{}

	for _, file := range files {
		matched := false
		for _, pattern := range patterns {
			matched, _ = doublestar.Match(path.Join(prefix, pattern), file)

			if matched {
				excluded = append(excluded, file)
				break
			}
		}

		if !matched {
			filtered = append(filtered, file)
		}
	}

	return filtered, excluded, nil
}

func Write(dir string, config *types.AnalysisConfig) error {
	path := filepath.Join(dir, "analysis_config.json")
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}
