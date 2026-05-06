package converter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DocSyncer struct {
	Mappings map[string][]string
	BaseDir  string
}

func NewDocSyncer(mappingInput string) (*DocSyncer, error) {
	var data []byte
	var err error
	
	mappingInput = strings.TrimSpace(mappingInput)
	if strings.HasPrefix(mappingInput, "{") {
		data = []byte(mappingInput)
	} else {
		data, err = os.ReadFile(mappingInput)
		if err != nil {
			return nil, fmt.Errorf("failed to read mapping file: %w", err)
		}
	}

	var mappings map[string][]string
	if err := json.Unmarshal(data, &mappings); err != nil {
		return nil, fmt.Errorf("failed to parse mapping: %w", err)
	}

	baseDir, _ := os.Getwd()
	return &DocSyncer{
		Mappings: mappings,
		BaseDir:  baseDir,
	}, nil
}

func (s *DocSyncer) Execute() error {
	errors := []string{}
	
	for dest, patterns := range s.Mappings {
		found := false
		var source string
		var isDir bool
		
		destFile := filepath.Base(dest)
		if destFile == "" || destFile == "." || destFile == "/" {
			errors = append(errors, fmt.Sprintf("destination must specify a file: %s", dest))
			continue
		}
		
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("invalid regexp %s: %w", pattern, err)
			}
			
			if src, dir, err := s.findMatch(re); err == nil && src != "" {
				source = src
				isDir = dir
				found = true
				break
			}
		}
		
		if !found {
			errors = append(errors, fmt.Sprintf("no match found for destination %s", dest))
			continue
		}
		
		if isDir {
			expectedFile := filepath.Join(source, destFile)
			if _, err := os.Stat(expectedFile); err != nil {
				errors = append(errors, fmt.Sprintf("directory %s does not contain required file %s", source, destFile))
				continue
			}
			targetDir := filepath.Dir(dest)
			if err := s.copyDir(source, targetDir); err != nil {
				return fmt.Errorf("failed to copy directory %s to %s: %w", source, targetDir, err)
			}
		} else {
			actualDest := dest
			if strings.HasSuffix(dest, "/") {
				actualDest = filepath.Join(dest, "index.md")
			}
			if err := s.copyFile(source, actualDest); err != nil {
				return fmt.Errorf("failed to copy %s to %s: %w", source, actualDest, err)
			}
		}
		
		fmt.Printf("Synced: %s -> %s\n", source, dest)
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("sync errors:\n%s", strings.Join(errors, "\n"))
	}
	
	return nil
}

func (s *DocSyncer) findMatch(re *regexp.Regexp) (string, bool, error) {
	var foundPath string
	var isDir bool
	
	err := filepath.Walk(s.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		relPath, _ := filepath.Rel(s.BaseDir, path)
		if relPath == "" {
			relPath = path
		}
		
		if re.MatchString(relPath) || re.MatchString(path) {
			if info.IsDir() {
				foundPath = path
				isDir = true
				return filepath.SkipDir
			} else if filepath.Ext(path) == ".md" {
				foundPath = path
				isDir = false
				return filepath.SkipDir
			}
		}
		
		return nil
	})
	
	if err != nil {
		return "", false, err
	}
	
	if foundPath == "" {
		return "", false, fmt.Errorf("no match found")
	}
	
	return foundPath, isDir, nil
}

func (s *DocSyncer) copyFile(source, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	
	dst, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer dst.Close()
	
	_, err = io.Copy(dst, src)
	return err
}

func (s *DocSyncer) copyDir(source, dest string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		
		destPath := filepath.Join(dest, relPath)
		
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		
		dstFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()
		
		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}