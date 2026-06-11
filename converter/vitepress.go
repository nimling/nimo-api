package converter

import (
	"encoding/json"
	"fmt"
	"github.com/nimling/nimo-api/utils"
	"github.com/nimling/nimo-api/vitepress"
	"gopkg.in/yaml.v3"
	"hash/fnv"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (n *OpenAPIConverter) WriteOpenAPISpec(outputDir string) error {
	specPath := filepath.Join(outputDir, n.CommonPrefix, fmt.Sprintf("%sspec.json", n.FilePrefix))
	err := n.writeOpenAPISpec(specPath)
	if err != nil {
		return fmt.Errorf("error: failed to write converter specs: %w", err)
	}

	fmt.Printf("Successfully wrote specs for vitepress to %s\n", specPath)
	return nil
}

func (n *OpenAPIConverter) WriteVitePressMarkdown(outputDir string) error {
	specPath := filepath.Join(outputDir, n.CommonPrefix, fmt.Sprintf("%sspec.json", n.FilePrefix))
	markdownPath := filepath.Join(outputDir, n.CommonPrefix)
	err := n.WriteMarkdown(specPath, markdownPath)
	if err != nil {
		return fmt.Errorf("error: failed to write markdown: %w", err)
	}

	fmt.Printf("Successfully wrote vitepress docs for spec %s\n", markdownPath)
	return nil
}

func (n *OpenAPIConverter) WriteMarkdown(specPath string, outputPath string) error {

	err := os.MkdirAll(outputPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create outputPath: %w", err)
	}

	specName := filepath.Base(specPath)

	data := struct {
		Prefix       string
		Title        string
		Description  string
		SpecFilePath string
		SpecFileName string
		FilePrefix   string
	}{
		Prefix:       strings.Trim(n.CommonPrefix, "/"),
		Title:        n.apiTitle,
		Description:  n.apiDescription,
		SpecFilePath: specPath,
		SpecFileName: specName,
		FilePrefix:   n.FilePrefix,
	}

	fileContent, err := utils.ExecuteTemplate("tags", oaTagsTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to execute markdown template: %w", err)
	}

	tagPath := path.Join(outputPath, "[tag].md")
	if err = os.WriteFile(tagPath, []byte(fileContent), 0644); err != nil {
		return fmt.Errorf("failed to write [tag].md]: %w", err)
	}
	LogWrite(tagPath, len(fileContent))

	fileContent, err = utils.ExecuteTemplate("paths", oaPathsTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to execute markdown template: %w", err)
	}

	pathsPath := path.Join(outputPath, "[tag].paths.js")
	if err = os.WriteFile(pathsPath, []byte(fileContent), 0644); err != nil {
		return fmt.Errorf("failed to write index.md: %w", err)
	}
	LogWrite(pathsPath, len(fileContent))

	if n.WriteIntroduction {
		fileContent, err = utils.ExecuteTemplate("introduction", oaIntroductionTemplate, data)
		if err != nil {
			return fmt.Errorf("failed to execute markdown template: %w", err)
		}

		introPath := path.Join(outputPath, fmt.Sprintf("%sintroduction.md", n.FilePrefix))
		if err = os.WriteFile(introPath, []byte(fileContent), 0644); err != nil {
			return fmt.Errorf("failed to write introduction.md: %w", err)
		}
		LogWrite(introPath, len(fileContent))
	}

	return nil
}

func (n *OpenAPIConverter) writeOpenAPISpec(outputPath string) error {

	err := os.MkdirAll(path.Dir(outputPath), 0755)
	if err != nil {
		return fmt.Errorf("failed to create outputPath: %w", err)
	}

	var raw interface{}
	yamlData, err := yaml.Marshal(n.doc)
	if err != nil {
		return fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	if err := yaml.Unmarshal(yamlData, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal YAML to interface: %w", err)
	}

	// Now marshal to JSON
	jsonData, err := json.MarshalIndent(raw, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}
	LogWrite(outputPath, len(jsonData))

	return nil
}

func getIconForTitle(title string) string {
	// Comprehensive list of professional and tech-relevant icons
	icons := []string{
		// Tech & Development
		"⚡", "🔌", "🖥️", "💻", "📱", "⌨️", "🖱️", "🖨️", "💾", "💿", "📀", "🗄️", "🔋", "🔌",

		// Data & Analytics
		"📊", "📈", "📉", "📋", "📑", "📝", "📏", "📐", "🗂️", "📁", "📂", "🗃️", "📤", "📥",

		// Security & Access
		"🔐", "🔒", "🔓", "🔑", "🗝️", "🛡️", "⚔️", "🔏", "🔗", "🔍", "🔎", "👁️", "📡", "🛰️",

		// Communication & Networking
		"🌐", "🔄", "↔️", "↕️", "🔃", "🔄", "📨", "📧", "📩", "📫", "📪", "📬", "📭", "📮",

		// Infrastructure & Cloud
		"☁️", "⚡", "🏢", "🏗️", "🌉", "🌁", "🗼", "🏛️", "🏰", "🎪", "🎯", "🎲", "🎮", "🎨",

		// Tools & Utilities
		"🔧", "🔨", "⚒️", "🛠️", "⛏️", "⚙️", "🔩", "⚡", "💡", "🔦", "🔆", "🔅", "📌", "📍",

		// Time & Schedule
		"⏱️", "⏲️", "⏰", "🕐", "🕑", "🕒", "🕓", "🕔", "🕕", "🕖", "🕗", "🕘", "🕙", "🕚",

		// Business & Commerce
		"💰", "💴", "💵", "💶", "💷", "💸", "💳", "🏦", "🏪", "🏬", "🏢", "📈", "📉", "📊",

		// Documents & Content
		"📄", "📃", "📜", "📯", "📚", "📖", "📘", "📙", "📔", "📕", "📓", "📒", "📑", "📋",

		// Alerts & Notifications
		"🔔", "🔕", "📢", "📣", "⚠️", "⛔", "🚫", "🔰", "✅", "❌", "❗", "❕", "❓", "❔",

		// Collaboration & Users
		"👥", "👤", "🤝", "🤲", "👐", "🙌", "👏", "🤜", "🤛", "✊", "👊", "🤝", "💪", "🧠",

		// Deployment & Operations
		"🚀", "✈️", "🛸", "🛫", "🛬", "🚁", "🚢", "🚂", "🚃", "🚄", "🚅", "🚇", "🚉", "🚊",

		// Monitoring & Health
		"💚", "💛", "❤️", "💔", "🩺", "🌡️", "💉", "💊", "🧬", "🔬", "🔭", "📡", "📶", "〽️",

		// Miscellaneous Tech
		"🎮", "🕹️", "🎲", "🎯", "🎨", "🎭", "🎪", "🎬", "🎼", "🎵", "🎶", "🎧", "🎤", "🎹",
	}

	// Create a deterministic hash of the title
	hash := fnv.New32a()
	hash.Write([]byte(title))
	hashValue := hash.Sum32()

	// Use the hash to select an icon
	return icons[hashValue%uint32(len(icons))]
}

func (n *OpenAPIConverter) WriteVitePressFeatures(outputPath string) error {
	content, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("failed to read index.md: %w", err)
	}

	parts := strings.Split(string(content), "---")
	if len(parts) < 3 {
		return fmt.Errorf("invalid frontmatter format in index.md")
	}

	var frontmatter *vitepress.Frontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err != nil {
		return fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	featureLink := path.Join("/api", n.CommonPrefix)
	if n.WriteIntroduction {
		featureLink = path.Join(featureLink, "introduction.md")
	}

	feature := vitepress.Feature{
		Link:    &featureLink,
		Title:   n.apiTitle,
		Details: n.apiDescription,
		Icon:    getIconForTitle(n.apiTitle),
	}

	if frontmatter.Features == nil {
		frontmatter.Features = []vitepress.Feature{feature}
	} else {
		found := false
		for i, existingFeature := range frontmatter.Features {
			if existingFeature.Link != nil && *existingFeature.Link == featureLink {
				frontmatter.Features[i] = feature
				found = true
				break
			}
		}
		
		if !found {
			frontmatter.Features = append(frontmatter.Features, feature)
		}
	}

	updatedFrontmatter, err := yaml.Marshal(frontmatter)
	if err != nil {
		return fmt.Errorf("failed to marshal updated frontmatter: %w", err)
	}

	newContent := fmt.Sprintf("---\n%s---\n%s", string(updatedFrontmatter), strings.TrimPrefix(strings.Join(parts[2:], "---"), "\n"))
	if err := os.WriteFile(outputPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write updated index.md: %w", err)
	}
	LogWrite(outputPath, len(newContent))

	return nil
}

// Helper function to get the link value from a feature node
func getFeatureLink(feature *yaml.Node) string {
	if feature.Kind != yaml.MappingNode {
		return ""
	}

	for i := 0; i < len(feature.Content); i += 2 {
		if feature.Content[i].Value == "link" {
			return feature.Content[i+1].Value
		}
	}
	return ""
}
