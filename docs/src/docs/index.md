---
title: Overview
---

**Lobby** is a simple, yet highly performant, network traffic load balancer based on Linux [nftables](https://wiki.nftables.org/wiki-nftables/). It is designed to run as a portable binary app on Linux systems in amd64 and arm64 architectures.

## Getting Started (without docker)

!!! info
    In case you have :simple-docker: docker installed in the same host as where you're planning to run Lobby, follow [this](#getting-started-with-docker) instead.

On a amd64 or arm64 Linux system, simply:

``` bash title="Get Lobby"
wget -q -O - https://ipbuff.com/getLobby | sh
```

!!! success ""

    And that's it! It is all it takes to get Lobby available on your system

This command downloads and runs a script which makes Lobby available on your current directory by downloading the Lobby binary to your current folder which becomes accessible with `./lobby`

It also sets up a demo configuration file in `./lobby.conf`. The demo configuration will get Lobby to load balance all TCP traffic hitting the Lobby host on port `8082` to a cloud server used for Lobby testing at `lobby-test.ipbuff.com:8081`, `lobby-test.ipbuff.com:8082` and `lobby-test.ipbuff.com:8083` in [`round-robin`](features.md#round-robin) distribution mode.

The only thing you have to ensure before running Lobby is that IP forwarding is enabled on your host system. In case you need help with this, check [this](https://www.baeldung.com/linux/kernel-ip-forwarding) article about it.

You should be able to get Lobby up and running as a **privileged user** with:

``` bash title="Run Lobby"
./lobby
```

!!! warning "In case of failure"

    If you get a permissions error, make sure you run `lobby` as 'root' user or with `sudo` or `doas`. To learn more on how to run as unprivileged user check [here](installation.md#permissions)

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

    If your only option is to test locally, consider using this [docker setup](installation.md#docker-network).

To stop Lobby just send it an `SIGINT` signal with a `Ctrl + c` from the terminal that is running it or through the `kill` or `pkill` commands.

In order to setup the load balancing as per your needs, feel free to edit the `./lobby.conf` config file. You'll be able to find the full configuration reference [here](configuration.md). 

With Lobby running, when it receives a `SIGHUP` signal, it will reprocess the config file and reconfigure the load balancing based on the updated config file contents.

## Getting Started (with docker)

!!! note
    Before using the examples below, please make sure to run the commands from a directory in which you want to store the Lobby configuration file `lobby.conf`

[This demo configuration](https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/demo.conf) will be used for test purposes.

``` title="Get Lobby Demo Config File"
wget -q \
  -O lobby.conf \
  https://raw.githubusercontent.com/ipbuff/lobby/main/docs/examples/demo.conf
```

In order to run Lobby on docker use the command below:

``` 
docker run --rm -d \
  --name lobby  \
  --cap-add=NET_ADMIN --cap-add=NET_RAW \
  -v $(pwd)/lobby.conf:/lobby/lobby.conf \
  -p 8082:8082 \
  ipbuff/lobby:latest
```

Now, to test, let's check the load balancing to the Lobby test servers with:

``` bash
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

Finally, when you're done with testing, just stop the Lobby container with:

```
docker stop lobby
```

In order to setup the load balancing as per your needs, feel free to edit the `./lobby.conf` config file. You'll be able to find the full configuration reference [here](configuration.md). 

With Lobby running, when it receives a `SIGHUP` signal, it will reprocess the config file and reconfigure the load balancing based on the updated config file contents. To send a SIGHUP to the Lobby container just:

``` 
docker kill -s SIGHUP lobby
```

## Learning More
In case you're interested in learning more about Lobby, you'll be able to find here more about its [feature](features.md) set, [installation](installation.md) options, [configuration](configuration.md) reference, [tutorials](tutorials.md) and how to get additional [support](support.md).
