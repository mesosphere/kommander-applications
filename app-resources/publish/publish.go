package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	confluenceBaseURL = "https://confluence.eng.nutanix.com:8443"
	confluencePageID  = "411939285" //Confluence Page ID
	csvPath           = "../generate/management_resource.csv"
)

// read token from env
var apiToken = os.Getenv("CONFLUENCE_API_TOKEN")

func init() {
	if apiToken == "" {
		log.Fatal("Missing required environment variable: CONFLUENCE_API_TOKEN")
	}
}

type ConfluenceContent struct {
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
}

func getCurrentPageVersion() (int, error) {
	url := fmt.Sprintf("%s/rest/api/content/%s?expand=version", confluenceBaseURL, confluencePageID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("getting page version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("error fetching version, status %d: %s", resp.StatusCode, body)
	}

	var content ConfluenceContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return 0, fmt.Errorf("decoding version response: %w", err)
	}

	return content.Version.Number, nil
}

func updateConfluencePage(content string, newVersion int) error {
	url := fmt.Sprintf("%s/rest/api/content/%s", confluenceBaseURL, confluencePageID)

	payload := map[string]interface{}{
		"id":    confluencePageID,
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

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("updating page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update page, status %d: %s", resp.StatusCode, body)
	}

	return nil
}

func generateResourceHTML(csvPath string) (string, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return "", fmt.Errorf("opening CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("reading CSV: %w", err)
	}

	var htmlTable strings.Builder
	htmlTable.WriteString("<h2>Management Application Resource Usage</h2>\n")
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

func main() {
	// build the HTML from the CSV
	resourceHTML, err := generateResourceHTML(csvPath)
	if err != nil {
		log.Fatalf("error generating resource HTML: %v", err)
	}
	fullContent := resourceHTML

	// fetch current page version
	version, err := getCurrentPageVersion()
	if err != nil {
		log.Fatalf("error getting page version: %v", err)
	}

	// update Confluence
	if err := updateConfluencePage(fullContent, version+1); err != nil {
		log.Fatalf("error updating Confluence page: %v", err)
	}

	log.Println("✅ Confluence page updated successfully.")
}
