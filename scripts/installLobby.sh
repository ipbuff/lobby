#!/bin/sh
SCRIPT_NAME='installLobby'

LOBBY_BIN_URL='https://github.com/ipbuff/lobby/releases/latest/download'
LOBBY_AMD64_BIN_URL="${LOBBY_BIN_URL}/lobby-linux-amd64"
LOBBY_ARM64_BIN_URL="${LOBBY_BIN_URL}/lobby-linux-arm64"
LOBBY_DEMO_CONFIG_URL='https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/demo.conf'
LOBBY_SERVICE_URL='https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/lobby.service'

LOBBY_BIN_DIR='/usr/local/bin'
LOBBY_BIN_NAME='lobby'
LOBBY_BIN_PATH=${LOBBY_BIN_DIR}/${LOBBY_BIN_NAME}
LOBBY_CONF_DIR='/etc/lobby'
LOBBY_CONF_NAME='lobby.conf'
LOBBY_CONF_PATH=${LOBBY_CONF_DIR}/${LOBBY_CONF_NAME}
LOBBY_ROOT_SERVICE_PATH='/etc/systemd/system/lobby.service'

INIT=$(cat /proc/1/comm) # init system. systemd/other
USE_WGET=1               # 0 is false, anything else is true

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No color

print_introl() {
    printf '+-----------------------------------------------+\n'
    printf '|   Lobby - A Load Balancer based on nftables   |\n'
    printf '|                                               |\n'
    printf '|        Lobby systemd service installer        |\n'
    printf '+-----------------------------------------------+\n'
    printf '\n'
}

print_intros() {
    printf 'Lobby - A Load Balancer based on nftables\n'
    printf '\n'
    printf 'Lobby systemd service installer\n'
    printf '\n'
}

intro() {
    if tput cols > /dev/null 2>&1; then
        cols=$(tput cols)
        if [ "${cols}" -ge 50 ]; then
            print_introl
        else
            print_intros
        fi
    else
        print_intros
    fi
}

failPrint() {
    printf '\n%s cancelled\n' ${SCRIPT_NAME}
}

parseArgs() {
    if [ "$1" = "" ]; then
        OWNER=root
    else
        if id "$1" > /dev/null 2>&1; then
            OWNER=$1
        else
            printf '%bThe provided username '\''%s'\'' was not found. Provide an existing username%b\n' "$RED" "$1" "$NC"
            failPrint
            exit 1
        fi
    fi
    printf 'Lobby will be installed for '\''%s'\'' user\n' "$OWNER"
}

checkDeps() {
    if [ ! "$INIT" = "systemd" ]; then
        printf '%bNot a '\''systemd'\'' init type system. This install script can'\''t be used%b\n' "$RED" "$NC"
        failPrint
        exit 1
    fi

    if [ "$(id -u)" -ne 0 ]; then
        printf '%bThis installer must be run as '\''root'\'' user%b\n' "$RED" "$NC"
        failPrint
        exit 1
    fi

    if ! wget --version > /dev/null 2>&1; then
        printf ''\''wget'\'' not available. Will try '\''curl'\'' instead\n'
        USE_WGET=0
        if ! curl --version > /dev/null 2>&1; then
            printf ''\''curl'\'' also not available\n'
            printf '\n'
            printf '%b'\''curl'\'' or '\''wget'\'' are dependencies for this script%b\n' "$RED" "$NC"
            failPrint
            exit 1
        fi
    fi

}

checkArch() {
    # echo 'Checking system architecture compatibility'
    ARCH=$(uname -a | awk '{ print $(NF-1) }')
    if [ "$ARCH" = "aarch64" ]; then
        DOWNLOAD_URL="$LOBBY_ARM64_BIN_URL"
    elif [ "$ARCH" = "x86_64" ]; then
        DOWNLOAD_URL="$LOBBY_AMD64_BIN_URL"
    else
        printf '\n'
        printf '%bLobby %s is incompatible with %s%b\n' "$RED" "$LOBBY_VERSION" "$ARCH" "$NC"
        failPrint
        exit 1
    fi
    # echo "  Check successful. System is '$ARCH'"
}

download() {
    if [ "$USE_WGET" -ne 0 ]; then
        if ! wget -q -O "$1" "$2"; then
            return 1
        fi
    else
        if ! curl -L -s -o "$1" "$2"; then
            return 1
        fi
    fi

    return 0
}

downloadLobbyBin() {
    printf 'Downloading Lobby binary for %s system to %s\n' "$ARCH" "$LOBBY_BIN_PATH"
    if ! download ${LOBBY_BIN_PATH} ${DOWNLOAD_URL}; then
        printf '\n'
        printf 'Failed to download the Lobby binary from %s\n' "$DOWNLOAD_URL"
        failPrint
        exit 1
    fi

    chown "${OWNER}:${OWNER}" "$LOBBY_BIN_PATH"
    chmod 770 ${LOBBY_BIN_PATH}
    setcap 'cap_net_admin,cap_net_raw+ep' ${LOBBY_BIN_PATH}
}

prepConfig() {
    printf 'Downloading Lobby demo config file to %s\n' "$LOBBY_CONF_PATH"

    mkdir $LOBBY_CONF_DIR > /dev/null 2>&1
    chown "${OWNER}:${OWNER}" "$LOBBY_CONF_DIR"
    chmod 770 ${LOBBY_CONF_DIR}

    if [ ! -f ${LOBBY_CONF_PATH} ]; then
        if ! download ${LOBBY_CONF_PATH} ${LOBBY_DEMO_CONFIG_URL}; then
            printf '\n'
            printf 'Failed to download the Lobby demo config file from %s\n' "$LOBBY_DEMO_CONFIG_URL"
            failPrint
            exit 1
        fi
    fi

    chown "${OWNER}:${OWNER}" ${LOBBY_CONF_PATH}
    chmod 770 ${LOBBY_CONF_PATH}
}

prepSystemd() {
    if [ "$OWNER" = 'root' ]; then
        printf 'Downloading Lobby systemd service unit file to %s\n' "$LOBBY_ROOT_SERVICE_PATH"
        if [ ! -f $LOBBY_ROOT_SERVICE_PATH ]; then
            if ! download $LOBBY_ROOT_SERVICE_PATH $LOBBY_SERVICE_URL; then
                printf '\n'
                printf 'Failed to download the Lobby systemd service unit file from %s\n' "$LOBBY_SERVICE_URL"
                failPrint
                exit 1
            fi

            sed -i "s/{{ username }}/${OWNER}/" $LOBBY_ROOT_SERVICE_PATH
        fi
    else
        LOBBY_USER_SERVICE_DIR="/home/${OWNER}/.config/systemd/user"
        LOBBY_USER_SERVICE_PATH="${LOBBY_USER_SERVICE_DIR}/lobby.service"
        mkdir -p "$LOBBY_USER_SERVICE_DIR" > /dev/null 2>&1
        chown "${ONWER}:${OWNER}" "$LOBBY_USER_SERVICE_DIR"
        chmod 770 "$LOBBY_USER_SERVICE_DIR"

        printf 'Downloading Lobby systemd service unit file to %s\n' "${LOBBY_USER_SERVICE_PATH}"
        if [ ! -f "$LOBBY_USER_SERVICE_PATH" ]; then
            if ! download "$LOBBY_USER_SERVICE_PATH" "$LOBBY_SERVICE_URL"; then
                printf '\n'
                printf 'Failed to download the Lobby systemd service unit file from %s\n' "$LOBBY_SERVICE_URL"
                failPrint
                exit 1
            fi

            sed -i '/{{ username }}/d' "$LOBBY_USER_SERVICE_PATH"
        fi
    fi
}

outro() {
    printf '\n'
    printf '%bLobby was successfully installed%b\n' "$GREEN" "$NC"
    printf '\n'

    if [ "$OWNER" = 'root' ]; then
        printf 'So that the Lobby systemd service becomes available, you need to reload the systemd daemon with:\n'
        printf '  systemctl daemon-reload\n'
        printf '\n'
        printf 'The Lobby service can be started with:\n'
        printf '  systemctl start lobby\n'
        printf '\n'
        printf 'To start Lobby service always at system boot:\n'
        printf '  systemctl enable lobby\n'
        printf '\n'
        printf 'The Lobby service can be stopped with:\n'
        printf '  systemctl stop lobby\n'
        printf '\n'
        printf 'The Lobby config can be updated while Lobby is running with:\n'
        printf '  systemctl reload lobby\n'
        printf '\n'
        printf 'The Lobby service status can be checked with:\n'
        printf '  systemctl status lobby\n'
    else
        printf 'So that the Lobby systemd service becomes available, you need to reload the systemd daemon with:\n'
        printf '  systemctl --user daemon-reload\n'
        printf '\n'
        printf 'The Lobby service can be started with:\n'
        printf '  systemctl --user start lobby\n'
        printf '\n'
        printf 'To start Lobby service always at system boot:\n'
        printf '  systemctl --user enable lobby\n'
        printf '\n'
        printf 'The Lobby service can be stopped with:\n'
        printf '  systemctl --user stop lobby\n'
        printf '\n'
        printf 'The Lobby config can be updated while Lobby is running with:\n'
        printf '  systemctl --user reload lobby\n'
        printf '\n'
        printf 'The Lobby service status can be checked with:\n'
        printf '  systemctl --user status lobby\n'
    fi
    printf '\n'
}

intro

parseArgs "$1"

checkDeps

checkArch

downloadLobbyBin

prepConfig

prepSystemd

outro
