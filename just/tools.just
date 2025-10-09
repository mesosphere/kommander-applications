# Tools configuration and installation
# Converted from make/tools.mk

# Default versions
nkp_cli_version := "v0.0.0-dev.0"

# Get Go environment variables
goos := `go env GOOS`
goarch := `go env GOARCH`

# Binary paths
nkp_cli_bin := ".local/bin/nkp"

# Install NKP CLI binary
install-nkp-cli:
    #!/usr/bin/env bash
    mkdir -p .local/bin
    curl -fsSL https://s3.amazonaws.com/downloads.mesosphere.io/dkp/{{nkp_cli_version}}/nkp_{{nkp_cli_version}}_{{goos}}_amd64.tar.gz | tar xz -O > {{nkp_cli_bin}}
    chmod +x {{nkp_cli_bin}}

# Install all tools
install-tools: install-nkp-cli
    @echo "All tools installed successfully"
