#!/bin/bash

# Download go-acme/lego repository
download_and_extract() {
    local repo_url="https://github.com/go-acme/lego/archive/refs/heads/master.zip"
    local target_dir="$1"

    # Check if wget and unzip are installed
    if ! command -v wget >/dev/null || ! command -v unzip >/dev/null; then
        echo "Please ensure wget and unzip are installed."
        exit 1
    fi

    # Download and extract the source code
    wget -q -O lego-master.zip "$repo_url"
    unzip -q lego-master.zip -d "$target_dir"
    rm lego-master.zip
}

# Copy .toml files from providers to the specified directory
copy_toml_files() {
    local source_dir="$1/lego-master/providers"
    local target_dir="server/pkg/cert/config"

    # Remove the lego-master folder
    if [ ! -d "$target_dir" ]; then
        mkdir -p "$target_dir"
    fi

    # Copy .toml files
    find "$source_dir" -type f -name "*.toml" -exec cp {} "$target_dir" \;
}

# Remove the lego-master folder
remove_lego_master_folder() {
  local folder="$1/lego-master"
  rm -rf "$folder"
}

destination="./tmp"
download_and_extract "$destination"
copy_toml_files "$destination"
remove_lego_master_folder "$destination"
