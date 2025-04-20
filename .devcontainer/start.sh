#!/bin/bash

# install air
go install github.com/air-verse/air@latest

install zsh-autosuggestions
git clone https://github.com/zsh-users/zsh-autosuggestions ~/.oh-my-zsh/custom/plugins/zsh-autosuggestions

if ! grep -q "zsh-autosuggestions" ~/.zshrc; then
    # add zsh-autosuggestions to plugins list
    sed -i "/^plugins=(/s/)/ zsh-autosuggestions)/" ~/.zshrc
fi

# init nginx config dir
./.devcontainer/init-nginx.sh

# install app dependencies
echo "Installing app dependencies"
cd app && pnpm install -f
cd ..

# install docs dependencies
echo "Installing docs dependencies"
cd docs && pnpm install -f
cd ..
