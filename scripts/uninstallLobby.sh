#!/bin/sh
LOBBY_BIN_DIR='/usr/local/bin'
LOBBY_BIN_NAME='lobby'
LOBBY_BIN_PATH=${LOBBY_BIN_DIR}/${LOBBY_BIN_NAME}
LOBBY_CONF_DIR='/etc/lobby'
LOBBY_ROOT_SERVICE_PATH='/etc/systemd/system/lobby.service'

RED='\033[0;31m'
NC='\033[0m' # No color

intro() {
    echo Uninstalling Lobby
}

checkDeps() {
    if [ "$(id -u)" -ne 0 ]; then
        printf '%bThis uninstaller must be run as '\''root'\'' user%b\n\n' "$RED" "$NC"
        exit 1
    fi
}

deleteBin() {
    if [ -f "$LOBBY_BIN_PATH" ]; then
        rm -rf "$LOBBY_BIN_PATH"
    fi
}

deleteConf() {
    if [ -d "$LOBBY_CONF_DIR" ]; then
        rm -rf "$LOBBY_CONF_DIR"
    fi
}

deleteRootSysdSvs() {
    if [ -f "$LOBBY_ROOT_SERVICE_PATH" ]; then
        rm -rf "$LOBBY_ROOT_SERVICE_PATH"
    fi
}

deleteUserSysdSvs() {
    find /home/*/ -type f -path '*/.config/systemd/user/lobby.service' -exec rm {} \;
}

outro() {
    echo
    echo Uninstall completed
    echo
}

intro

checkDeps

deleteBin

deleteConf

deleteRootSysdSvs

deleteUserSysdSvs

outro
