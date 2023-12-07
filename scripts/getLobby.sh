#!/bin/sh
SCRIPT_NAME='getLobby'

LOBBY_BIN_URL='https://github.com/ipbuff/lobby/releases/latest/download'
LOBBY_AMD64_BIN_URL="${LOBBY_BIN_URL}/lobby-linux-amd64"
LOBBY_ARM64_BIN_URL="${LOBBY_BIN_URL}/lobby-linux-arm64"
LOBBY_DEMO_CONFIG_URL='https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/demo.conf'

CONF_PATH_LOCAL='./lobby.conf'
CONF_PATH_ETC='/etc/lobby/lobby.conf'

USE_WGET=1 # 0 is false, anything else is true
GREEN='\033[0;32m'
MAGENTA='\033[0;35m'
RED='\033[0;31m'
NC='\033[0m' # No color

print_introl() {
    printf '+-------------------------------------------------------+\n'
    printf '|       Lobby - A Load Balancer based on nftables       |\n'
    printf '|                                                       |\n'
    printf '| The Lobby binary will be downloaded to this directory |\n'
    printf '+-------------------------------------------------------+\n'
    printf '\n'
}

print_intros() {
    printf 'Lobby - A Load Balancer based on nftables\n'
    printf '\n'
    printf 'The Lobby binary will be downloaded to this directory\n'
    printf '\n'
}

intro() {
    if tput cols > /dev/null 2>&1; then
        cols=$(tput cols)
        if [ "${cols}" -ge 60 ]; then
            print_introl
        else
            print_intros
        fi
    else
        print_intros
    fi
}

failPrint() {
    printf '\n%s cancelled\n' "$SCRIPT_NAME"
}

checkDep() {
    # echo 'Checking script dependencies'
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
    # echo '  Dependencies successfully checked'
}

checkArch() {
    # echo 'Checking system architecture compatibility'
    ARCH=$(uname -a | awk '{ print $(NF-1) }')
    if [ "$ARCH" = "aarch64" ]; then
        DOWNLOAD_URL="${LOBBY_ARM64_BIN_URL}"
    elif [ "$ARCH" = "x86_64" ]; then
        DOWNLOAD_URL="${LOBBY_AMD64_BIN_URL}"
    else
        printf '\n'
        printf '%bLobby is incompatible with %s%b\n' "$RED" "$ARCH" "$NC"
        failPrint
        exit 1
    fi
    # echo "  Check successful. System is '$ARCH'"
}

checkLobbyDep() {
    IPV4_FWD=$(sysctl net.ipv4.ip_forward | awk '{ print $3 }')
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
    printf 'Downloading Lobby binary for %s system\n' "$ARCH"
    if ! download lobby ${DOWNLOAD_URL}; then
        printf '\n'
        printf 'Failed to download the Lobby binary from %s\n' "$DOWNLOAD_URL"
        rm -rf lobby
        failPrint
        exit 1
    fi

    chmod 770 ./lobby
}

prepConf() {
    if ! [ -f ${CONF_PATH_LOCAL} ]; then
        if ! [ -f ${CONF_PATH_ETC} ]; then
            # echo "Config file not found. Checked in '${CONF_PATH_LOCAL}' and '${CONF_PATH_ETC}'"
            # echo "  Creating config file in local path '${CONF_PATH_LOCAL}'"
            if ! download lobby.conf ${LOBBY_DEMO_CONFIG_URL}; then
                printf '\n'
                printf 'Failed to download the Lobby demo config file from %s\n' "$LOBBY_DEMO_CONFIG_URL"
                rm -rf lobby
                failPrint
                exit 1
            fi
        fi
    fi
}

outro() {
    printf '\n'
    printf 'Lobby was successfully prepared at this folder (%s)\n' "$(pwd)"
    printf '\n'
    printf 'A demo Lobby configuration for testing purposes has been prepared at '%s/lobby.conf'\n' "$(pwd)"
    printf 'Feel free to run Lobby with that demo configuration or by adjusting the config file\n'
    printf '\n'
    printf '%bConsider running Lobby as '\''root'\''.%b As an alternative, it is possible to run Lobby as an unprivileged user as long as the binary is given the '\''NET_ADMIN'\'' and '\''NET_RAW'\'' Linux capabilities. This can be achieved by running the following command as '\''root'\'':\n' "$MAGENTA" "$NC"
    printf '  # setcap '\''cap_net_admin,cap_net_raw+ep'\'' %s/lobby\n' "$(pwd)"
    printf '\n'
    if [ "$IPV4_FWD" -ne 1 ]; then
        printf '%bYour system seems not to have IP forwarding enabled.%b\nLobby will not be able to load balance traffic if the host doesn'\''t has IP forwarding enabled.\n' "$RED" "$NC"
        printf '\n'
    fi
    printf '%bLobby can be started with the following command:%b\n' "$GREEN" "$NC"
    printf '  './lobby'\n'
    printf '\n'
}

intro

checkDep

checkArch

checkLobbyDep

downloadLobbyBin

prepConf

outro
