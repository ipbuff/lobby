## Building from source
In order to build Lobby from source, it is required to do so from an environment prepared to [build go projects](https://go.dev/doc/install).

The build process is pretty simple. Go to a directory of your choosing and from there:

```
git clone --depth 1 https://github.com/ipbuff/lobby.git
cd lobby
make build
```

Following the successful build, the resulting binaries can be found in the `./bin` directory.

There will be two files:

  - `lobby-linux-amd64` for Linux system and amd64 architecture
  - `lobby-linux-arm64` for Linux system and arm64 architecture

## Getting the binaries and running Lobby
The Lobby binaries can be found on the [releases](https://github.com/ipbuff/lobby/releases) of the Lobby :simple-github: Github repo.

In case you have :simple-docker: docker installed on the same host as where you're planning to run Lobby, do not use the Lobby binaries and use the [docker](#docker) methods instead. The default :simple-docker: docker iptables/nftables rules drop all the forwarding traffic. In case you still want to proceed with the binary setup in a server which also has docker, then you'll have to edit the docker generated iptables/nftables rules.

### Download Lobby
The simplest method is to use the [`getLobby.sh`](https://github.com/ipbuff/lobby/blob/main/scripts/getLobby.sh) script which will detect your system and download the appropriate binary for your system. This script will also set a demo Lobby config file which can be used to test Lobby. This script is available [here](https://ipbuff.com/getLobby).

Start by creating a directory where you want to download the Lobby binaries to. For this example, `~/lobby` will be used. The directory can be created with:

``` bash title="Create directory to store Lobby"
LOBBY_DIR=~/lobby
mkdir $LOBBY_DIR && \
cd $LOBBY_DIR && \
echo Successfully created the lobby dir || \
echo Failed to create the lobby dir
```

A way to download the script and run it can be:

``` title="Get Lobby Binary and Config File"
wget -q -O - https://ipbuff.com/getLobby | sh
```

Now if you check the contents of the directory, you should find there the Lobby binary named `lobby` and the demo configuration file named `lobby.conf`.

``` bash title="List Directory Contents"
ls
```

### CLI Help
To get the command line interface (cli) help, which prints the flags available for Lobby, run Lobby with the `-h` flag.

```bash title="Print cli help"
./lobby -h
```

### Check Version
To check the Lobby version just run lobby with the `-v` flag. The version will be printed as result and the app will quit.

``` bash title="Check Lobby version"
./lobby -v
```

### Setting permissions
At this stage, you have to decide with which user you want to run Lobby with. Eventually the simplest way is to proceed as `root`, but we'll proceed this tutorial with the consideration that best practices should be followed even in introductory tutorials and in that case we'll assume that an unprivileged user is to be used.

The script executed at [this](#download-lobby) step has already ensured that the binary can only be executed by the owner user and group. Additionally, in order to run Lobby with an unprivileged user it is necessary to give permissions to the binary to run with a couple of linux capabilities which allow it to manage specifically the nftables locally. The capabilites are `NET_ADMIN` and `NET_RAW` and we'll follow the instructions described [here](installation.md#permissions) to set them for the Lobby binary with the following command executed by the `root` user:

``` bash title="Execute as root user"
setcap 'cap_net_admin,cap_net_raw+ep' lobby
```

The success of the command can be confirmed with:

``` bash
getcap lobby
```

!!! success

    With the expected output being:

    ``` bash
    lobby cap_net_admin,cap_net_raw=ep
    ```

From here on, it is possible to run Lobby as an unprivileged user.

Keep in mind that the Linux capabilities have to be set every time a new binary is built/downloaded - ie after every upgrade

### Running Lobby
Running Lobby without any flags will start the load balancer with the local config file named `lobby.conf`. In case `lobby.conf` is not found or is inaccessible, then the `/etc/lobby/lobby.conf` path will be attempted next. In case either of the files fails to open, Lobby will quit with error code 1.

!!! note
    On [this](#download-lobby) step, [this](https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/demo.conf) demo config file has been setup locally. In case you have followed a different method to set up the Lobby binaries, make sure that there is a Lobby configuration file at either the local directory of the Lobby binary or at `/etc/lobby/lobby.conf`

Lobby can be started simply with:

``` bash title="Start Lobby"
./lobby
```

#### Test Lobby
As Lobby doesn't load balances localhost traffic, the tests have to be performed from another machine targeting the Lobby host.

If you haven't changed anything on the demo config file, you should be able to successfully target your Lobby host on port `8082` **from another machine** to test if Lobby is working correctly. 

This can be achieved in many ways. One of them is with:

``` bash title="Test Lobby from another machine"
for i in {1..6}; do curl <IP of Lobby host>:8082; done
```

!!! success "Result"

    In case of success, this test will call the load balancer 6 times on port `8082` and Lobby will load balance the traffic in round-robin mode across 3 different Lobby test upstreams.

    The expected output is:

    ```
    8081
    8082
    8083
    8081
    8082
    8083
    ```

!!! warning "In case of failure"

    Make sure this test is performed from another machine as the traffic from the test machine is to be proxied through the Lobby host to the configured upstream. `curl localhost:8082` won't work and is expected to fail.

    If your only option is to test locally, consider using this [docker setup](#docker).

### Update Config
Lobby supports hot reloading of the configuration, which means that it is possible to update the configuration without having to stop and start the load balancer with the benefit of no traffic being temporarily lost in the process.

This is achieved by updating the configuration file in use by Lobby and sending a SIGHUP signal to the running process.

As an example, while Lobby is running, let's add another upstream to `target2` by appending the following config, which will add a new upstream to `target2`.

``` bash
echo "            - name: lobby-test-server4" >> lobby.conf
echo "              host: lobby-test.ipbuff.com" >> lobby.conf
echo "              port: 8084" >> lobby.conf
```

Now that the config file has been updated, we just need to send a `SIGHUP` signal to the running process of Lobby. This can be achieved with:

``` bash
pkill -SIGHUP lobby
```

Now that Lobby has reconfigured we can run the same test:

``` bash title="Test Lobby from another machine"
for i in {1..6}; do curl <IP of Lobby host>:8082; done
```

!!! success "Result"

    And now given we have one extra test upstream, the expected output is:

    ```
    8081
    8082
    8083
    8084
    8081
    8082
    ```

### Stopping Lobby
Lobby can be stopped with `Ctrl+c` on the terminal where it is running in the foreground or with a SIGINT signal to the running process with: 

``` bash
pkill -SIGINT lobby
```

### Specifying a Config File
Lobby supports specifying a path to a specific config file. This can be achieved with the `-c` flag. For example: 

``` bash
./lobby -c <path to config file>
```

### Verbosity Level
The default logging verbosity for Lobby is `Info`. It is possible to start Lobby with different levels of logging verbosity. This is achieved with the `-l` flag.

The possible logging levels are:

| Log Level       | Description                      | Flag string    | Displays |
|---------------- | -------------------------        | -------------- | -------- |
| Critical        | Fatal errors and similar         | `critical`     | `critical` |
| Warning         | Potentially problematic events   | `warning`      | `critical`/`warning` |
| Info            | Potentially relevant information | `info`         | `critical`/`warning`/`info` |
| Debug           | User debugging level             | `debug`        | `critical`/`warning`/`info`/`debug` |
| DebugVerbose    | Developer debugging level        | `debugverbose` | `critical`/`warning`/`info`/`debug`/`debugverbose` |

So, for instance to set the logging level to Debug level, start Lobby with:

``` bash 
./lobby -l debug
```

## Lobby systemd Service
Other than for testing purposes, it is recommended to run Lobby as a systemd service.

### Install Lobby (systemd)
[This](https://github.com/ipbuff/lobby/releases/latest/download/installLobby.sh) script provides a way to setup Lobby as a systemd service. It does the following:

  - Downloads the latest Lobby binary for the appropriate system
  - Places the binary in `/usr/local/bin/lobby` with conservative permissions
  - Sets a demo config file in `/etc/lobby/lobby.conf`
  - Creates a systemd service until file for Lobby 
    - at `/etc/systemd/system/lobby.service` for `root` user
    - otherwise at `~/.config/systemd/user/lobby.service` for other users

The script accepts a username as an argument in order to install Lobby with the appopriate permissions for the given user. In case no argument is provided, it is assumed that the installation is to be done for the `root` user.

The installation script has to be run as `root` user given it requires privileged permissions to set everything thing up.

#### As root user systemd service
In order to use the script to install Lobby for the `root` user:

``` bash title="Run as root user"
wget -q -O - https://ipbuff.com/installLobby | sh
```

Following the succesful completion of the installation script it is necessary to reload the systemd deamon in order to make the Lobby systemd service available. This can be achieved by running the following command as `root` user:

``` 
systemctl daemon-reload
```

Now you can start Lobby with:

``` 
systemctl start lobby
```

And in case you want Lobby to start automatically at system boot:

``` 
systemctl enable lobby
```

To stop the Lobby service:

``` 
systemctl stop lobby
```

And to refresh the Lobby configuration following config file changes:

``` 
systemctl reload lobby
```

##### Security Improvement
While leaving the systemd service managed by root, so it can be started automatically at every system boot, consider executing the Lobby binary with an unprivileged user.

For that purpose, create a specific user, give `rwx` permissions to it or its group to the `/etc/lobby/` directory and at least `x` to the binary `/usr/local/bin/lobby`.

Then to ensure that the systemd service runs Lobby with the desired user add the `User=changeMe` argument somewhere in the `[Service]` section of the Lobby systemd service unit file at `/etc/systemd/system/lobby.service`. From the previous example change `changeMe` to the user that was created and to be used.

Once the Lobby systemd service unit file has been updated with the `User` argument under the `[Service]` section, refresh systemd with:

``` 
systemctl daemon-reload
```

And for good measure stop, disable and enable and start again the Lobby systemd service:

``` 
systemctl stop lobby
systemctl disable lobby
systemctl enable lobby
systemctl start lobby
```

#### As non-root user systemd service
In order to use the script to install Lobby for the `john` user:

``` title="Run as root user and replace 'john' with your username"
wget -q -O - https://ipbuff.com/installLobby | sh -s john
```

Following the succesful completion of the installation script it is necessary to reload the systemd deamon in order to make the Lobby systemd service available. In case you've proceeded with the installation as a `non-root` user, this can be achieved by running the following command with the desired user: 

``` 
systemctl --user daemon-reload
```

Now you can start Lobby with:

```
systemctl --user start lobby
```

And in case you want Lobby to start automatically at **first user login**:

``` 
systemctl --user enable lobby
```

To stop the Lobby service:

``` 
systemctl --user stop lobby
```

And to refresh the Lobby configuration following config file changes:

``` 
systemctl --user reload lobby
```

### Uninstall Lobby (systemd)
It is recommended to stop and disable the Lobby systemd service before uninstalling Lobby.

In order to uninstall Lobby in case the [`installLobby.sh`](https://github.com/ipbuff/lobby/releases/latest/download/installLobby.sh) script was used, all that is necessary is to remove:

- the binary located in `/usr/local/bin/lobby`
- the configuration directory at `/etc/lobby/`
- eventually the systemd service unit files for the root user at `/etc/systemd/system/lobby.service`
- eventually the systemd service unit files for non-root users

A [script](https://github.com/ipbuff/lobby/releases/latest/download/uninstallLobby.sh) has been prepared for that. Before using it, backup your config file as it is removed as part of the uninstall process.

You can run it as a root user with:

!!! note
    Do not forget to stop and disable the Lobby systemd service beforehand

``` title="Run as root"
wget -q -O - https://ipbuff.com/uninstallLobby | sh
```

And then finally, complete with a systemd refresh with `systemctl daemon-reload`.

## Docker
This tutorial will be assuming your default docker network is a bridge network (as it is most likely the case and) as it happens by default after docker install. In case this is not the case, adjust the steps and commands accordingly.

Change directory to where you will be storing your Lobby config file before proceeding with this docker tutorial.

### Check Version
To start with, let's check the Lobby version from the container images hosted at [:simple-docker: docker hub](https://hub.docker.com/r/ipbuff/lobby).

``` 
docker run --rm \
  --name lobby \
  --cap-add=NET_ADMIN --cap-add=NET_RAW \
  ipbuff/lobby:latest \
  ./lobby -v
```

### Running Lobby on Docker
Running Lobby without any flags will start the load balancer with the local config file named `lobby.conf`.

Generate your own Lobby config file or download a demo config file like so:

``` bash
wget \
  -O lobby.conf \
  -q https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/demo.conf
```

Then Lobby can be started simply with:

``` 
docker run --rm -d \
  --name lobby \
  --cap-add=NET_ADMIN --cap-add=NET_RAW \
  -v $(pwd)/lobby.conf:/lobby/lobby.conf \
  -p 8082:8082 \
  ipbuff/lobby:latest
```

#### Test Lobby on Docker
If you haven't changed anything on the demo config file, you should be able to successfully target your port `8082` to test if Lobby is working correctly. 

This can be achieved in many ways. One of them is with:

``` bash title="Test Lobby from another machine"
for i in {1..6}; do curl localhost:8082; done
```

!!! success "Result"

    In case of success, this test will call the load balancer 6 times on port `8082` and Lobby will load balance the traffic in round-robin mode across 3 different Lobby test upstreams.

    The expected output is:

    ```
    8081
    8082
    8083
    8081
    8082
    8083
    ```

### Update Config on Docker
As in [here](#update-config), while Lobby is running, let's add another upstream to `target2` by appending the following config, which will add a new upstream to `target2`.

``` bash
echo "            - name: lobby-test-server4" >> lobby.conf
echo "              host: lobby-test.ipbuff.com" >> lobby.conf
echo "              port: 8084" >> lobby.conf
```

Now that the config file has been updated, we just need to send a `SIGHUP` signal to the running process of Lobby in the container. This can be achieved with:

``` 
docker kill -s SIGHUP lobby
```

Now that Lobby has reconfigured we can run the same test:

``` bash title="Test Lobby from another machine"
for i in {1..6}; do curl ${LOBBY_IP}:8082; done
```

!!! success "Result"

    And now given we have one extra test upstream, the expected output is:

    ```
    8081
    8082
    8083
    8084
    8081
    8082
    ```

### Stopping Lobby on Docker
Lobby can be stopped with `Ctrl+c` on the terminal where it is running in the foreground or with a SIGINT signal to the running process with: 

``` 
docker kill -s SIGINT lobby
```

### Verbosity level on Docker
As explained [here](#verbosity-level), for instance to set the logging level to Debug level, start Lobby with:

``` 
docker run --rm -d \
  --name lobby \
  --cap-add=NET_ADMIN --cap-add=NET_RAW \
  -v $(pwd)/lobby.conf:/lobby/lobby.conf \
  -p 8082:8082
  ipbuff/lobby:latest \
  ./lobby -l debug
```

### Things to Note
#### Container Ports
For [this](#running-lobby-on-docker) type of docker deployment, consider specifying a range of ports to be exposed for the Lobby docker container, instead of individual ones. This is in case future load balancing configuration requires load balancing on new/other ports. For instance, to expose from port 8000 to 9000 one could use the `-p 8000-9000:8000-9000` flag instead.

#### Production on Docker
For production deployments, consider creating a specific docker network to be used by exclusively by the Lobby containers and not subject to NAT'ing.

## Considerations for Production Deployments
The examples above are mostly useful for local testing or for home/small load applications. 

However, to host Lobby for production it is important to have in consideration the wider context of the deployment, such as:

- The traffic routing to the Lobby server host
- The traffic routing from the Lobby server host
- Lobby traffic isolation
- The Lobby server host resources
- Lobby redundancy
- etc

Nevertheless, for a high performance, high throughput and high bandwidth environment, consider the [systemd service](#lobby-systemd-service) or similar deployment type for Lobby in a host that is protected by a firewall and where the Lobby internet address is available at the Lobby host without NAT rules in between. The Lobby (or other load balancers) redundancy implementation depends on the use case and therefore cannot be subject to a generic recommendation in this tutorial.

In case of need for support on production deployments or operation, consider reaching out to the [project creator](https://igor.borisoglebski.com) for consultancy and professional services.

## Traffic Stats
This part of Lobby is still in its infancy and unfortunately in extremelly low capacity. Feel free to reach out to the [project creator](https://igor.borisoglebski.com) in case you're willing to support the acceleration of the development or feel free contribute with the capability through the [:simple-github:Github project](https://github.com/ipbuff/lobby).

However, the most basic measure has been implemented and can be checked through `nftables`. All targets are assigned a [nftables counter](https://wiki.nftables.org/wiki-nftables/index.php/Counters) which keeps track of the traffic incoming to each target. This can be checked for instance with:

``` bash
nft list counters
```

It outputs the packets and bytes incoming to each target.


This method will be further enhanced to keep track of more things, accessible through other methods and integratable with external systems such as prometheus.

## Generating Config
Generating the Lobby config should be relatively intuitive for anyone who has worked with [YAML](https://yaml.org/) structures before.

### One Target to Two Upstreams
Let's create a config file with Lobby listening TCP traffic on port 8082 and load balancing to two upstreams in round-robin distribution mode where one of the upstreams is expecting traffic on IP 1.1.1.1 and port 80, while the other one on IP 1.1.1.2 and also port 80. 

Every config file starts with:

``` yaml
lb:
  - engine: nftables
    targets:
```

We start by adding the target definition which is where Lobby is listening for that type of traffic with:

``` yaml
lb:
  - engine: nftables
    targets:
      - name: toCloudflare
        protocol: tcp
        port: 8082
```

Now that we have defined where Lobby is listening, we need to specify how the traffic is distributed:

``` yaml
lb:
  - engine: nftables
    targets:
      - name: toCloudflare
        protocol: tcp
        port: 8082
        upstream_group:
          name: cloudflareServers
          distribution: round-robin
          upstreams:
```

At this stage, this config file is not valid yet, because we haven't specified the upstream servers to where the traffic will be load balanced. So, we need to add them:

``` yaml
lb:
  - engine: nftables
    targets:
      - name: toCloudflare
        protocol: tcp
        port: 8082
        upstream_group:
          name: cloudflareServers
          distribution: round-robin
          upstreams:
            - name: cloudflareServer1
              host: 1.1.1.1
              port: 80
            - name: cloudflareServer2
              host: 1.1.1.2
              port: 80
```

And that's it! The most basic load balancing config file was created. For further references check [here](configuration.md).
