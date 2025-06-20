package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"encoding/csv"
	"path/filepath"

	"app-resources/constants"
	"app-resources/confluence"
)

var csvPaths = []string{
	"../generate/management_resource.csv",
}

const (
	confluencePageID = "411939285"
)

var (
	FlagConfluenceUsername = os.Getenv("CONFLUENCE_USERNAME")
	FlagConfluenceAPIToken = os.Getenv("CONFLUENCE_APITOKEN")
)

func generateResourceHTML(csvPath string) (string, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return "", fmt.Errorf("opening CSV file %s: %w", csvPath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("reading CSV %s: %w", csvPath, err)
	}

	var htmlTable strings.Builder
	htmlTable.WriteString(fmt.Sprintf("<h2>%s</h2>\n", formatTitle(csvPath)))
	htmlTable.WriteString("<table><tbody>\n")

	for i, row := range records {
		htmlTable.WriteString("<tr>")
		for _, cell := range row {
			tag := "td"
			if i == 0 {
				tag = "th"
			}
			htmlTable.WriteString(fmt.Sprintf("<%s>%s</%s>", tag, cell, tag))
		}
		htmlTable.WriteString("</tr>\n")
	}
	htmlTable.WriteString("</tbody></table>")
	return htmlTable.String(), nil
}

func formatTitle(path string) string {
	// Extract filename and remove extension
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	words := strings.Split(base, "_")
	for i, w := range words {
		words[i] = strings.Title(w)
	}
	return strings.Join(words, " ")
}

func getCurrentPageVersion(client *confluence.Client, pageID string) (int, error) {
	url := fmt.Sprintf("https://%s/rest/api/content/%s?expand=version", clientDomain(client), pageID)

	var result struct {
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
	}

	if err := client.Get(url, &result); err != nil {
		return 0, fmt.Errorf("getting page version: %w", err)
	}

	return result.Version.Number, nil
}

func updateConfluencePage(client *confluence.Client, content string, pageID string, newVersion int) error {
	url := fmt.Sprintf("https://%s/rest/api/content/%s", clientDomain(client), pageID)

	payload := map[string]interface{}{
		"id":    pageID,
		"type":  "page",
		"title": "Component and Application Versions",
		"version": map[string]interface{}{
			"number": newVersion,
		},
		"body": map[string]interface{}{
			"storage": map[string]string{
				"value":          content,
				"representation": "storage",
			},
		},
	}

	var result any
	if err := client.Put(url, payload, &result); err != nil {
		return fmt.Errorf("failed to update page: %w", err)
	}

	return nil
}

func clientDomain(client *confluence.Client) string {
	return strings.Split(client.Domain(), ":")[0]
}

func main() {
	apiToken := os.Getenv(constants.FlagConfluenceAPIToken)
	if apiToken == "" {
		log.Fatal("❌ Missing required environment variable: CONFLUENCE_APITOKEN")
	}

	domain := strings.TrimPrefix(constants.ConfluenceDomainNew, "https://")
	client := confluence.NewClientWithBearer(domain, apiToken)

	var allHTML strings.Builder
	for _, path := range csvPaths {
		log.Printf("📄 Generating HTML content from CSV: %s\n", path)
		html, err := generateResourceHTML(path)
		if err != nil {
			log.Fatalf("❌ Error generating HTML from %s: %v", path, err)
		}
		allHTML.WriteString(html + "<br/><br/>")
	}

	log.Println("🔍 Fetching current Confluence page version...")
	version, err := getCurrentPageVersion(client, confluencePageID)
	if err != nil {
		log.Fatalf("❌ Error getting page version: %v", err)
	}

	log.Println("📤 Updating Confluence page...")
	if err := updateConfluencePage(client, allHTML.String(), confluencePageID, version+1); err != nil {
		log.Fatalf("❌ Error updating Confluence page: %v", err)
	}

	log.Println("✅ Confluence page updated successfully.")
}
