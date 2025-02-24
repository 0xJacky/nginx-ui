#!/bin/bash

# Version validation regex pattern
VALID_VERSION_REGEX='^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9\.]+)?$'

# Prompt for version input
while true; do
    read -p "Enter version number: " VERSION

    # Remove 'v' prefix for validation
    if [[ "${VERSION#v}" =~ $VALID_VERSION_REGEX ]]; then
        # Show confirmation prompt with original input
        echo "You entered version: ${VERSION}"
        read -p "Is this correct? [Y/n] " confirm
        case ${confirm,,} in
            y|yes|"") break ;;
            n|no)
                echo "Restarting version input..."
                continue
                ;;
            *)
                echo "Invalid input, please answer Y/n"
                continue
                ;;
        esac
    else
        echo "Error: Invalid version format. Please use semantic versioning (e.g. 2.0.0, v2.0.1-beta.1)"
    fi
done

# Cross-platform compatible sed command
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s/\"version\": \".*\"/\"version\": \"${VERSION#v}\"/" app/package.json
else
    sed -i "s/\"version\": \".*\"/\"version\": \"${VERSION#v}\"/" app/package.json
fi
echo "Updated package.json to version ${VERSION#v}"

# Build app
echo "Building app..."
cd app && pnpm build
if [ $? -ne 0 ]; then
    echo "Error: Build failed"
    exit 1
fi
cd ..

# Run go generate
echo "Generating Go code..."
go generate ./...
if [ $? -ne 0 ]; then
    echo "Error: go generate failed"
    exit 1
fi

echo "Version update and generation completed successfully"
