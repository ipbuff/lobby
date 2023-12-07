A sample configuration file with all the configuration possibilies can be found below.

Refer to [features](features.md) for additional details on each configuration parameter.

``` yaml title="Example config file with comments"
lb:
  - engine: nftables
    targets:
      - name: target1                     # unique target name
        # A target listening on TCP port 8081, using 3 upstreams to load balance traffic in round-robin mode
        protocol: tcp                     # transport protocol. Only tcp supported for now
        port: 8081                        # unique target port for a given protocol
        upstream_group:
          name: t1ug1                     # unique upstream_group name
          distribution: round-robin       # ug traffic distribution mode. Only round-robin supported for now
          upstreams:
            - name: t1upstream1           # unique upstream name
              # An upstream hosted at 1.1.1.1 IP address and port 80
              # No active health-checking and therefore the upstream will be considered always as available to receive traffic
              host: 1.1.1.1               # upstream host. IP or FQDN
              port: 80                    # upstream port
            - name: t1upstream2           # unique upstream name
              # An upstream hosted at 1.1.1.2 IP address and port 80
              # Active health-checking is performed on TCP port 80, every 10 seconds. 3 consecutive successful probes are required to consider the upstream as available. A probe will fail after 2 seconds timeout
              # The upstream will be considered as available when the load balancer starts
              host: 1.1.1.2               # upstream host. IP or FQDN
              port: 80                    # upstream port
              health_check:
                protocol: tcp             # health-heck protocol. Only tcp supported for now
                port: 80                  # health-check port. It can be different from the upstream port
                start_available: true     # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 10      # seconds. Max value: 65536
                  timeout: 2              # seconds. Max value: 256
                  success_count: 3        # amount of successful health checks for upstream to become available
            - name: t1upstream3           # unique upstream name
              # An upstream hosted at 1.1.1.3 IP address and port 80
              # Active health-checking is performed on TCP port 443, every 10 seconds. 5 consecutive successful probes are required to consider the upstream as available. A probe will fail after 1 seconds timeout
              # The upstream will be considered as unavailable when the load balancer starts
              protocol: tcp               # transport protocol. Only tcp supported for now
              host: 1.1.1.3               # upstream host. IP or FQDN
              port: 80                    # upstream port
              health_check:
                protocol: tcp             # health-heck protocol. Only tcp supported for now
                port: 443                 # health-check port. It can be different from the upstream port
                start_available: false    # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 10      # seconds. Max value: 65536
                  timeout: 1              # seconds. Max value: 256
                  success_count: 5        # amount of successful health checks to become active
      - name: target2                     # unique target name
        # A target listening on TCP port 8082, using 3 upstreams to load balance traffic in round-robin mode
        protocol: tcp                     # transport protocol. Only tcp supported for now
        port: 8082                        # unique target port for a given protocol
        upstream_group:                   # upstream_group to be used for target
          name: t2ug1                     # unique upstream_group name
          distribution: round-robin       # ug traffic distribution mode. Only round-robin supported for now
          upstreams:
            - name: lobby-test-server1    # unique upstream name
              # An upstream hosted at lobby-test.ipbuff.com IP address and port 8081
              # The system DNS's are used to resolve the upstream host FQDN
              # No active health-checking and therefore the upstream will be considered always as available to receive traffic
              protocol: tcp               # transport protocol. Only tcp supported for now
              host: lobby-test.ipbuff.com # upstream host. IP or FQDN
              port: 8081                  # upstream port
            - name: lobby-test-server2    # unique upstream name
              # An upstream hosted at lobby-test.ipbuff.com IP address and port 8082
              # The 1.1.1.1, 8.8.8.8 and 2606:4700::1111 DNS's are used to resolve the upstream host FQDN. The DNS will be re-queried every 300 seconds
              # Active health-checking is performed on TCP port 8082, every 30 seconds. 3 consecutive successful probes are required to consider the upstream as available. A probe will fail after 1 seconds timeout
              # The upstream will be considered as available when the load balancer starts
              protocol: tcp               # transport protocol. Only tcp supported for now
              host: lobby-test.ipbuff.com # upstream host. IP or FQDN
              port: 8082                  # upstream port
              dns:                        # include in case you want to use specific DNS to resolve the fqdn host address. If host is IPv4 or IPv6 this setting will not have any effect. In case this mapping is not present the OS resolvers will be used
                servers:                  # dns address list. Queries will be done sequentially in case of failure
                  - 1.1.1.1               # cloudflare IPv4 DNS
                  - 8.8.8.8               # google IPv4 DNS. Used if 1.1.1.1 DNS fails to resolve
                  - 2606:4700::1111       # cloudflare IPv6 DNS. Used if 1.1.1.1 and 8.8.8.8 DNS fail to resolve
                ttl: 300                  # custom ttl can be specified to overwrite the DNS response TTL
              health_check:               # don't include the health-check mapping or leave it empty to disable health-check. upstreams will be considered alwasy as active when health-checks are not enabled
                protocol: tcp             # health-heck protocol. Only tcp supported for now
                port: 8082                # health-check port. It can be different from the upstream port
                start_available: true     # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 30      # seconds. Max value: 65536
                  timeout: 1              # seconds. Max value: 256
                  success_count: 3        # amount of successful health checks to become active
            - name: lobby-test-server3    # unique upstream name
              # An upstream hosted at lobby-test.ipbuff.com IP address and port 8083
              # The 1.1.1.1, 8.8.8.8 and 2606:4700::1111 DNS's are used to resolve the upstream host FQDN
              # No active health-checking and therefore the upstream will be considered always as available to receive traffic
              protocol: tcp               # transport protocol. Only tcp supported for now
              host: lobby-test.ipbuff.com # upstream host. IP or FQDN
              port: 8083                  # upstream port
              dns:                        # include in case you want to use specific DNS to resolve the fqdn host address. If host is IPv4 or IPv6 this setting will not have any effect. In case this mapping is not present the OS resolvers will be used
                servers:                  # dns address list. Queries will be done sequentially in case of failure. The DNS will be re-requeried according to the TTL received in the DNS response
                  - 1.1.1.1               # cloudflare IPv4 DNS
                  - 8.8.8.8               # google IPv4 DNS. Used if 1.1.1.1 DNS fails to resolve
                  - 2606:4700::1111       # cloudflare IPv6 DNS. Used if 1.1.1.1 and 8.8.8.8 DNS fail to resolve
```

``` yaml title="Example config file without comments"
lb:
  - engine: nftables
    targets:
      - name: target1
        protocol: tcp
        port: 8081
        upstream_group:
          name: t1ug1
          distribution: round-robin
          upstreams:
            - name: t1upstream1
              host: 1.1.1.1
              port: 80
            - name: t1upstream2
              host: 1.1.1.2
              port: 80
              health_check:
                protocol: tcp
                port: 80
                start_available: true
                probe:
                  check_interval: 10
                  timeout: 2
                  success_count: 3
            - name: t1upstream3
              protocol: tcp
              host: 1.1.1.3
              port: 80
              health_check:
                protocol: tcp
                port: 443
                start_available: false
                probe:
                  check_interval: 10
                  timeout: 1
                  success_count: 5
      - name: target2
        protocol: tcp
        port: 8082
        upstream_group: 
          name: t2ug1
          distribution: round-robin
          upstreams:
            - name: lobby-test-server1
              protocol: tcp
              host: lobby-test.ipbuff.com
              port: 8081
            - name: lobby-test-server2
              protocol: tcp
              host: lobby-test.ipbuff.com
              port: 8082
              dns: 
                servers: 
                  - 1.1.1.1
                  - 8.8.8.8
                  - 2606:4700::1111
                ttl: 300
              health_check:
                protocol: tcp
                port: 8082
                start_available: true
                probe:
                  check_interval: 30
                  timeout: 1
                  success_count: 3
            - name: lobby-test-server3
              protocol: tcp
              host: lobby-test.ipbuff.com
              port: 8083
              dns: 
                servers: 
                  - 1.1.1.1
                  - 8.8.8.8
                  - 2606:4700::1111
```

!!! note
    As Lobby's config file is based on YAML, it will fail to load with a malformatted YAML config file.
