## Requirements
### System
Up to the current version, Lobby was prepared to run on `Linux/amd64` and `Linux/arm64` systems.

### Kernel
The Kernel must have the `nf_tables` module loaded or available to be loaded given that Lobby uses nftables to orchestrate the load balancing.

IPv4 and IPv6 forwarding must also be enabled through the kernel parameters. In most Linux systems `sysctl` can be used to check and enable/disable IP forwarding. Check [here](https://www.baeldung.com/linux/kernel-ip-forwarding), for instance, in case you need help with regards to managing IP forwarding in your systems.

### Permissions
The Lobby binary must have the `NET_ADMIN` and `NET_RAW` [linux capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html) set to `Permitted` and `Effective`.

In most systems this can be achieved with the following command executed by the `root` user:

``` bash
setcap 'cap_net_admin,cap_net_raw+ep' /path/to/lobby
```

Also ensure that the binary can (only?) be executed by the appropriate user and group with `chown` and `chmod`.

## Binary
Pre-built binaries are available [here](https://github.com/ipbuff/lobby/releases/) and are named:

  - for linux/amd64 systems: `lobby-linux-amd64`
  - for linux/arm64 systems: `lobby-linux-arm64`

As an alternative to direct binaries, the [releases](https://github.com/ipbuff/lobby/releases/) also include scripts to download the binary and a demo configuration file.

The binary can be located anywhere, but consider placing it named `lobby` in one of your `$PATH` directories such as `/usr/local/bin` or `/usr/bin`.

Lobby has its load balancing rules set through a config file. Lobby will look for a config file named `lobby.conf` in its local directory and if not found in its local directory, it will then try to open it from `/etc/lobby/lobby.conf`. If you've placed Lobby in one of your `$PATH` directories, then place the configuration file in `/etc/lobby/lobby.conf`. It is also possible to specify the config file with the `-c` flag such as `lobby -c /path/to/config/file.yaml`.

## Building from Source
The Lobby source code is publicly available at [:simple-github: Github](https://github.com/ipbuff/lobby).

In order to build from source, make sure you have the [go environment set up](https://go.dev/doc/install) and then simply clone the repo and use `make build` from within the repo directory.

There's a tutorial [here](tutorials.md#building-from-source).

## Docker
Lobby containers are published on [:simple-docker: docker hub](https://hub.docker.com/r/ipbuff/lobby).

The containers have been built with ["Distroless"](https://github.com/GoogleContainerTools/distroless) images. Therefore, it will not possible to run anything else other than Lobby on those containers.

!!! note
    Before using the examples below, please make sure to run the commands from a directory in which the Lobby configuration file `lobby.conf` exists and contains valid configuration. 

    Otherwise, adjust the commands accordingly.

    [This demo configuration](https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/demo.conf) could be used for test purposes.

In order to run Lobby on docker while using a separate network namespace for the container and exposing locally some of the ports consider the command below:

``` bash
docker run -d \
  --name lobby  \
  --cap-add=NET_ADMIN --cap-add=NET_RAW \
  -v $(pwd)/lobby.conf:/lobby/lobby.conf \
  -p 8081:8081 -p 8082:8082 \
  ipbuff/lobby:latest
```

The `-d` flag runs the container in [dettached](https://docs.docker.com/language/golang/run-containers/#run-in-detached-mode) mode.

The `name` flag sets the container name.

The `--cap-add` provides the `NET_ADMIN` and `NET_RAW` linux capabilites to the container processes.

The `-v` flag mounts the local `./lobby.conf` config file in the container on the `/lobby/lobby.conf` path.

The `-p` flag is used to map a local port to a port within the container. 

`ipbuff/lobby:latest` uses the latest image for the `ipbuff/lobby` container hosted on docker hub.

!!! note

    This method whilst it might be convenient, it is potentially a non-optimal setup for production environments as docker sets up a NAT rule to get the traffic from the local port to the container address and port. This adds extra computation when compared to other options. Additionally, it also complicates on how to expose additional ports as result of configuration changes.

## systemd
Running Lobby as a systemd requires setting the lobby service file in `/etc/systemd/system/`. 

[Here](https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/lobby.service)'s a sample systemd service file which can be used after replacing the `User`, `WorkingDirectory` and `ExecStart` parameters :

``` systemd title="/etc/systemd/system/lobby.service"
[Unit]
Description=Lobby Load Balancer
After=network.target

[Service]
Type=simple
User={{ username }}
ExecStart=/usr/local/bin/lobby
ExecStop=/bin/kill -s SIGINT $MAINPID
ExecReload=/bin/kill -s SIGHUP $MAINPID
TimeoutStartSec=0
RestartSec=2
Restart=always
StartLimitBurst=3
StartLimitInterval=60s

[Install]
WantedBy=multi-user.target
```

Make sure that if the user is not set to `root`, the lobby binary has the required [Linux capabilities](#permissions).

Once the `/etc/systemd/system/lobby.service` file is created, consider setting it with the appropriate ownership and permissions. If unsure, use:

``` bash
chown root:root /etc/systemd/system/lobby.service
chmod 644 /etc/systemd/system/lobby.service
```

And then finally reload the systemd config with:

``` bash
systemctl daemon-reload
```

The lobby service then can be enabled with:

``` bash
systemctl start lobby
```

And it should be enabled in case you wish it to start every time the system boots:

``` bash
systemctl enable lobby
```

In order to stop Lobby:

``` bash
systemctl stop lobby
```

It is possible to refresh the Lobby configuration after editing the config file with:

``` bash
systemctl reload lobby
```

There's a [script](https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/lobby.service) which helps with the Lobby systemd service setup. Its usage is documented in the [tutorials](tutorials.md#lobby-systemd-service).
