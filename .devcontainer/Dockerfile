FROM mcr.microsoft.com/devcontainers/base:noble

# Combine installation steps for Nginx and Go to avoid repetitive update/cleanup commands
RUN apt-get update && \
  apt-get install -y --no-install-recommends curl gnupg2 ca-certificates lsb-release ubuntu-keyring jq cloc software-properties-common && \
  \
  # Add PPA repository for nginx-extras
  add-apt-repository -y ppa:ondrej/nginx-mainline && \
  \
  # Update package information and install Nginx-extras
  apt-get update && \
  apt-get install -y --no-install-recommends nginx nginx-extras inotify-tools file && \
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
  rm go.tar.gz

RUN cp -rp /etc/nginx /etc/nginx.orig

# Set PATH to include Go installation and default go install binary location
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

ENV NGINX_UI_WORKING_DIR=/var/run/

# set zsh as default shell
RUN chsh -s $(which zsh)
