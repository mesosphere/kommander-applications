package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	// Path to management apps file
	managementAppsFile := "management_apps.txt"
	allowedApps := make(map[string]bool)
	customNames := make(map[string]string)

	// Load app mappings
	file, err := os.Open(managementAppsFile)
	if err != nil {
		log.Fatal("Error opening management_apps.txt: ", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		customName := parts[0]
		appName := parts[1]
		appNameLower := strings.ToLower(appName)

		allowedApps[appNameLower] = true
		customNames[appNameLower] = customName
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("Error reading management_apps.txt: ", err)
	}

	// Prepare output file and write headers
	outputFile := "management_resource.csv"
	fileOutput, err := os.Create(outputFile)
	if err != nil {
		log.Fatal("Error creating output file: ", err)
	}
	defer fileOutput.Close()

	writer := bufio.NewWriter(fileOutput)
	writer.WriteString("CustomAppName,App,Version,CPU,Memory\n")

	// Walk the services directory recursively
	err = filepath.Walk("../../services", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only interested in files named cm.yaml inside defaults folder
		if info.IsDir() || !strings.HasSuffix(path, "defaults/cm.yaml") {
			return nil
		}

		// Example path: ../services/kommander/0.16.0/defaults/cm.yaml
		parts := strings.Split(path, string(os.PathSeparator))
		if len(parts) < 4 {
			return nil
		}

		app := parts[len(parts)-4]
		version := parts[len(parts)-3]
		appLower := strings.ToLower(app)

		if _, exists := allowedApps[appLower]; exists {
			cpu, memory := extractResourceData(path)

			if cpu == "" {
				cpu = "N/A"
			}
			if memory == "" {
				memory = "N/A"
			}

			customAppName := app
			if customName, exists := customNames[appLower]; exists {
				customAppName = customName
			}

			writer.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s\n", customAppName, app, version, cpu, memory))
		}
		return nil
	})
	if err != nil {
		log.Fatal("Error walking services directory: ", err)
	}

	writer.Flush()

	// Sort the output file
	sortCSV(outputFile)

	// Print the output location
	fmt.Printf("Output saved to %s\n", outputFile)
}

// Function to extract CPU and memory from cm.yaml
func extractResourceData(filePath string) (string, string) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Println("Error reading cm.yaml: ", err)
		return "", ""
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(fileContent, &configMap); err != nil {
		log.Println("Error parsing YAML: ", err)
		return "", ""
	}

	data, ok := configMap["data"].(map[string]interface{})
	if !ok {
		return "", ""
	}

	valuesYaml, ok := data["values.yaml"].(string)
	if !ok {
		return "", ""
	}

	cpu := extractResource(valuesYaml, "cpu")
	memory := extractResource(valuesYaml, "memory")

	return cpu, memory
}

// Helper function to extract CPU or Memory using regex
func extractResource(valuesYaml, resource string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?m)^\s*%s:\s*(\S+)`, resource))
	matches := re.FindStringSubmatch(valuesYaml)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// Sort the CSV output file alphabetically by CustomAppName
func sortCSV(outputFile string) {
	fileContent, err := os.ReadFile(outputFile)
	if err != nil {
		log.Fatal("Error reading CSV file: ", err)
	}

	lines := strings.Split(string(fileContent), "\n")
	if len(lines) < 2 {
		return
	}

	header := lines[0]
	dataLines := lines[1:]
	dataLines = removeEmpty(dataLines)
	sort.Strings(dataLines)

	fileOutput, err := os.Create(outputFile)
	if err != nil {
		log.Fatal("Error creating output file: ", err)
	}
	defer fileOutput.Close()

	writer := bufio.NewWriter(fileOutput)
	writer.WriteString(header + "\n")
	for _, line := range dataLines {
		writer.WriteString(line + "\n")
	}
	writer.Flush()
}

// Helper to remove empty lines
func removeEmpty(lines []string) []string {
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}
