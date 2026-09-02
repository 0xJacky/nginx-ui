#!/bin/bash

# setup git for claude
git config --global gpg.format ssh
git config --global user.signingkey ~/.ssh/id_ed25519.pub
git config --global commit.gpgsign true

# install air
go install github.com/air-verse/air@latest

# install zsh-autosuggestions
git clone https://github.com/zsh-users/zsh-autosuggestions ~/.oh-my-zsh/custom/plugins/zsh-autosuggestions

if ! grep -q "zsh-autosuggestions" ~/.zshrc; then
    # add zsh-autosuggestions to plugins list
    sed -i "/^plugins=(/s/)/ zsh-autosuggestions)/" ~/.zshrc
fi

# init nginx config dir
./.devcontainer/init-nginx.sh

# install workspace dependencies
echo "Installing workspace dependencies"
bun ci
