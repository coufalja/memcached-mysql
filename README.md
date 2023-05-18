# memcached-mysql

Memcached mysql proxy mimicks the behaviour of MySQL memcached plugin but in a standalone process.

## Usage

To build the proxy, run:

```bash
make
```

To run the proxy, run:

```bash
# Configuration file can be passed in via the `--config` flag.
# Default value is `./config.yaml`.
./memcached-proxy --config path/to/config.yaml

# The path to the configuration file can be passed in as an environment variable as well:
CONFIG=path/to/config.yaml ./memcached-proxy
```

Address of the MySQL server, address of the memcached proxy, as well as mapping can be configured
in the configuration file. For the full specification of the configurable values, see the `Config`
struct in the [`config/config.go`](./config/config.go) file.
