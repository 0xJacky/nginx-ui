FROM mcr.microsoft.com/devcontainers/base:jammy

# Combine installation steps for Nginx and Go to avoid repetitive update/cleanup commands
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl gnupg2 ca-certificates lsb-release ubuntu-keyring jq && \
    \
    # Configure the Nginx repository
    curl https://nginx.org/keys/nginx_signing.key | gpg --dearmor > /usr/share/keyrings/nginx-archive-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/nginx-archive-keyring.gpg] http://nginx.org/packages/mainline/ubuntu $(lsb_release -cs) nginx" \
        > /etc/apt/sources.list.d/nginx.list && \
    printf "Package: *\nPin: origin nginx.org\nPin: release o=nginx\nPin-Priority: 900\n" \
        > /etc/apt/preferences.d/99nginx && \
    \
    # Update package information and install Nginx
    apt-get update && \
    apt-get install -y --no-install-recommends nginx inotify-tools file && \
    \
    # Automatically retrieve the latest stable Go version and install it,
    # download the appropriate binary based on system architecture (amd64 or arm64)
    GO_VERSION=$(curl -sSL "https://golang.org/dl/?mode=json" | \
        jq -r 'map(select(.stable)) | .[0].version' | sed 's/^go//') && \
    ARCH=$(dpkg --print-architecture) && \
    if [ "$ARCH" = "arm64" ]; then \
      GO_ARCH=linux-arm64; \
    else \
      GO_ARCH=linux-amd64; \
    fi && \
    echo "Installing Go version: ${GO_VERSION} for architecture: ${GO_ARCH}" && \
    curl -sSL "https://golang.org/dl/go${GO_VERSION}.${GO_ARCH}.tar.gz" -o go.tar.gz && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz && \
    \
    # Remove jq and clean up to reduce image size
    apt-get remove -y jq && \
    apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN cp -rp /etc/nginx /etc/nginx.orig

# Set PATH to include Go installation and default go install binary location
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

# Install air with go install (requires Go 1.23 or higher)
RUN go install github.com/air-verse/air@latest

# set zsh as default shell
RUN chsh -s $(which zsh)
