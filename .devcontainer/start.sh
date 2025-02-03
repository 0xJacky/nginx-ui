#!/bin/bash

# install zsh-autosuggestions
git clone https://github.com/zsh-users/zsh-autosuggestions ~/.oh-my-zsh/custom/plugins/zsh-autosuggestions

if ! grep -q "zsh-autosuggestions" ~/.zshrc; then
    # add zsh-autosuggestions to plugins list
    sed -i "/^plugins=(/s/)/ zsh-autosuggestions)/" ~/.zshrc
fi

# init nginx config dir
if [ "$(ls -A /etc/nginx)" = "" ]; then
    echo "Initialing Nginx config dir"
    cp -rp /etc/nginx.orig/* /etc/nginx/
    echo "Initialed Nginx config dir"
fi

# install app dependencies
echo "Installing app dependencies"
cd app && pnpm install -f
# Build app
pnpm build
cd ..

# install docs dependencies
echo "Installing docs dependencies"
cd docs && pnpm install -f
cd ..
