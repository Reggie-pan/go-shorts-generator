#!/bin/bash
set -e

# 如果 /assets/fonts 存在且有檔案，則安裝字型
if [ -d "/assets/fonts" ] && [ "$(ls -A /assets/fonts)" ]; then
    echo "Found custom fonts in /assets/fonts, installing..."
    mkdir -p /usr/share/fonts/custom
    cp /assets/fonts/* /usr/share/fonts/custom/
    fc-cache -f
    echo "Custom fonts installed."
fi

# 啟動應用程式
exec /app/server
