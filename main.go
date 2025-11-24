package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type ReadmeConfig struct {
	Name        string
	Env         string
	Value       string
	Default     string
	Description string
}

type ReadmeConfigs []ReadmeConfig

type Container struct {
	XMLName     xml.Name          `xml:"Container"`
	Version     string            `xml:"version,attr"`
	Name        string            `xml:"Name"`
	Repository  string            `xml:"Repository"`
	Registry    string            `xml:"Registry"`
	Branch      Branch            `xml:"Branch"`
	Network     string            `xml:"Network"`
	Shell       string            `xml:"Shell"`
	WebUI       string            `xml:"WebUI"`
	Privileged  bool              `xml:"Privileged"`
	Support     string            `xml:"Support"`
	Project     string            `xml:"Project"`
	Overview    string            `xml:"Overview"`
	Category    string            `xml:"Category"`
	Beta        bool              `xml:"Beta"`
	Icon        string            `xml:"Icon"`
	TemplateURL string            `xml:"TemplateURL"`
	Maintainer  Maintainer        `xml:"Maintainer"`
	Changes     string            `xml:"Changes"`
	Requires    string            `xml:"Requires"`
	DonateText  string            `xml:"DonateText"`
	DonateLink  string            `xml:"DonateLink"`
	Config      []ContainerConfig `xml:"Config"`
}

type Branch struct {
	Tag            string `xml:"Tag"`
	TagDescription string `xml:"TagDescription"`
}

type Maintainer struct {
	WebPage string `xml:"WebPage"`
}

type ContainerConfig struct {
	Name        string `xml:"Name,attr"`
	Target      string `xml:"Target,attr"`
	Default     string `xml:"Default,attr"`
	Mode        string `xml:"Mode,attr,omitempty"`
	Description string `xml:"Description,attr"`
	Type        string `xml:"Type,attr"`
	Display     string `xml:"Display,attr"`
	Required    bool   `xml:"Required,attr"`
	Mask        bool   `xml:"Mask,attr"`
	Value       string `xml:",chardata"`
}

func LoadAndParseLocales() (string, error) {
	readme, err := http.Get("https://raw.githubusercontent.com/damongolding/immich-kiosk/main/assets/locales.md")
	if err != nil {
		return "", fmt.Errorf("error getting file: %w", err)
	}

	defer readme.Body.Close()
	content, err := io.ReadAll(readme.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	markdownContent := string(content)

	var locales []string

	lines := strings.Split(markdownContent, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "-") {
			continue
		}

		d := strings.Split(line, "\"")
		locales = append(locales, d[1])
	}

	return strings.Join(locales, "|"), nil
}

func LoadAndParseReadmeTables() (ReadmeConfigs, error) {
	readme, err := http.Get("https://raw.githubusercontent.com/damongolding/immich-kiosk/main/unraid.md")
	if err != nil {
		return nil, fmt.Errorf("error getting file: %w", err)
	}

	defer readme.Body.Close()
	content, err := io.ReadAll(readme.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Convert to string
	markdownContent := string(content)

	// Find both tables
	firstTableStart := strings.Index(markdownContent, "| **yaml**")
	if firstTableStart == -1 {
		return nil, fmt.Errorf("first table not found in README")
	}

	// Find the second table start after the first table
	secondTableStart := strings.Index(markdownContent[firstTableStart+1:], "| **yaml**")
	if secondTableStart == -1 {
		return nil, fmt.Errorf("second table not found in README")
	}
	// Adjust the second table start position to be relative to the full content
	secondTableStart += firstTableStart + 1

	var tables ReadmeConfigs

	// Parse first table
	firstTableContent := extractTableContent(markdownContent[firstTableStart:secondTableStart])
	tables = ParseMarkdownTable(firstTableContent)

	// Parse second table
	secondTableContent := extractTableContent(markdownContent[secondTableStart:])
	secondTable := ParseMarkdownTable(secondTableContent)

	tables = append(tables, secondTable...)

	return tables, nil
}

// Helper function to extract table content
func extractTableContent(content string) string {
	var tableContent strings.Builder
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || !strings.HasPrefix(trimmedLine, "|") {
			break
		}
		line = strings.Replace(line, "\\|", "@", -1)
		tableContent.WriteString(line + "\n")
	}
	return tableContent.String()
}

func ParseMarkdownTable(markdownContent string) ReadmeConfigs {
	var configs ReadmeConfigs

	// Split content into lines
	lines := strings.Split(markdownContent, "\n")

	// Skip the header row and separator row (first two lines)
	for _, line := range lines[2:] {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Split the line by | and trim spaces
		parts := strings.Split(line, "|")

		if strings.TrimSpace(parts[2]) == "N/A" {
			continue
		}

		// Skip the first empty part (before first |) and trim each part
		if len(parts) >= 5 {

			name := strings.TrimSpace(parts[1])
			env := strings.TrimSpace(parts[2])
			value := strings.TrimSpace(parts[4])
			defaultValue := strings.TrimSpace(parts[3])
			description := strings.TrimSpace(parts[5])

			// Name formatting
			if len(strings.Split(name, "]")) > 1 {
				name = strings.Replace(strings.Split(name, "]")[0], "[", "", -1)
			}

			name = strings.Replace(name, "_", " ", -1)
			name = cases.Title(language.BritishEnglish).String(name)

			// Description formatting
			description = strings.TrimSpace(strings.Split(description, "See [")[0])

			if value == "\"\"" {
				value = ""
			}

			// Value formatting
			switch defaultValue {
			case "bool":
				defaultValue = "false|true"
				if value == "true" {
					defaultValue = "true|false"
				}

			case "[]string":
				defaultValue = ""

			case "int", "float":
				defaultValue = value

			case "string":
				defaultValue = ""
				if value != "" {
					defaultValue = value
				}
			}

			if strings.Contains(defaultValue, "@") {
				values := strings.Split(defaultValue, "@")
				for i, v := range values {
					values[i] = strings.TrimSpace(v)
				}
				defaultValue = strings.Join(values, "|")
				value = values[0]
			}

			if value == "[]" {
				value = ""
			}

			if strings.HasPrefix(defaultValue, "[") {
				defaultValue = ""
			}

			config := ReadmeConfig{
				Name:        name,
				Env:         env,
				Value:       value,
				Default:     defaultValue,
				Description: description,
			}
			configs = append(configs, config)
		}
	}

	return configs
}

func generateConfigElements(config ReadmeConfigs) []ContainerConfig {
	var configs []ContainerConfig

	locales, err := LoadAndParseLocales()
	if err != nil {
		log.Fatalf("Error loading and parsing locales: %v", err)
	}

	// Add base configs
	configs = append(configs, ContainerConfig{
		Name:        "Language",
		Target:      "LANG",
		Default:     locales,
		Description: "The language code for Kiosk to use",
		Type:        "Variable",
		Display:     "always",
		Required:    false,
		Mask:        false,
		Value:       "en_GB",
	})

	configs = append(configs, ContainerConfig{
		Name:        "Timezone",
		Target:      "TZ",
		Default:     "",
		Description: "The timezone for Kiosk to use",
		Type:        "Variable",
		Display:     "always",
		Required:    false,
		Mask:        false,
		Value:       "Europe/London",
	})

	configs = append(configs, ContainerConfig{
		Name:        "Web UI Port",
		Target:      "3000",
		Default:     "3000",
		Mode:        "tcp",
		Description: "Container Port: 3000",
		Type:        "Port",
		Display:     "always",
		Required:    false,
		Mask:        false,
		Value:       "3000",
	})

	configs = append(configs, ContainerConfig{
		Name:        "Config File",
		Target:      "/config.yaml",
		Default:     "/mnt/user/appdata/immich_kiosk/config.yaml",
		Mode:        "rw",
		Description: "Config file for application. Remove this if using environmental variables instead.",
		Type:        "Path",
		Display:     "always",
		Required:    false,
		Mask:        false,
	})

	for _, c := range config {

		// Create config element
		config := ContainerConfig{
			Name:        c.Name,
			Target:      c.Env,
			Default:     c.Default,
			Description: c.Description,
			Type:        "Variable",
			Display:     "always",
			Required:    false,
			Mask:        false,
			Value:       c.Value,
		}

		if strings.EqualFold(c.Env, "KIOSK_IMMICH_API_KEY") || strings.EqualFold(c.Env, "KIOSK_IMMICH_URL") {
			config.Required = true
		}

		configs = append(configs, config)
	}

	return configs
}

func main() {
	container := Container{
		Version:    "2",
		Name:       "ImmichKiosk",
		Repository: "ghcr.io/damongolding/immich-kiosk:latest",
		Registry:   "https://github.com/damongolding/immich-kiosk/pkgs/container/immich-kiosk",
		Branch: Branch{
			Tag:            "latest",
			TagDescription: "Latest stable release",
		},
		Network:     "bridge",
		Shell:       "sh",
		WebUI:       "http://[IP]:[PORT:3000]",
		Privileged:  false,
		Support:     "https://github.com/damongolding/immich-kiosk/issues",
		Project:     "https://github.com/damongolding/immich-kiosk",
		Overview:    "Immich Kiosk is a lightweight slideshow for running on kiosk devices and browsers that uses Immich as a data source.",
		Category:    "Other: MediaApp:Photos",
		Beta:        true,
		Icon:        "https://raw.githubusercontent.com/damongolding/immich-kiosk/main/assets/logo.png",
		TemplateURL: "https://raw.githubusercontent.com/damongolding/immich-kiosk-unraid/main/immich_kiosk.xml",
		Maintainer: Maintainer{
			WebPage: "https://github.com/damongolding",
		},
		Requires:   "A reachable Immich server that is running version v1.127.0 or above. See the project page for more information: https://github.com/damongolding/immich-kiosk?tab=readme-ov-file",
		DonateText: "If this project has been helpful to you and you wish to support me, you can do so with the link below 🙂.",
		DonateLink: "https://www.buymeacoffee.com/damongolding",
	}

	configs, err := LoadAndParseReadmeTables()
	if err != nil {
		log.Fatalf("Error loading and parsing README: %v", err)
	}

	container.Config = generateConfigElements(configs)

	output, err := xml.MarshalIndent(container, "", "    ")
	if err != nil {
		panic(err)
	}

	final := []byte(xml.Header + string(output))
	err = os.WriteFile("immich_kiosk.xml", final, 0644)
	if err != nil {
		panic(err)
	}
}
