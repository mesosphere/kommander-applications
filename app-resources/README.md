# Management Applications Resource Reporting

This module streamlines the process of generating and publishing CPU and memory resource usage reports for management applications used in the Kommander platform.

## Directory Structure
app-resources/
├── go.mod
├── go.sum
├── generate/
│ ├── generate_report.go # Script to collect CPU/Memory info
│ ├── management_apps.txt # List of management apps to include
├── publish/
│ ├── publish.go # Script to publish the generated report to Confluence

## Scope

This directory addresses the requirement defined in the task:  
**Create a report of resource usage (CPU/Memory) for management applications and publish it to Confluence.**

The solution involves:
- Parsing resource data (CPU and memory) from app `cm.yaml` files.
- Generating a CSV (`management_resource.csv`) summarizing the data.
- Converting the CSV to HTML and publishing it to a specific Confluence page **URL:https://confluence.eng.nutanix.com:8443/pages/viewpage.action?spaceKey=KAR&title=Component+and+Application+Versions**

## Usage

The process is automated via GitHub Actions and includes the following steps:

1. **Generate Report**  
   Parses the `cm.yaml` files for the apps listed in `management_apps.txt` and generates `management_resource.csv`.

2. **Publish Report**  
   Converts the CSV to HTML and publishes it to the target Confluence page via API.
