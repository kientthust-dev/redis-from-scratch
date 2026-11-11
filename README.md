# Redis From Scratch in Go

A Redis-inspired in-memory data store implemented in Go, built to explore network programming, Linux I/O multiplexing, the RESP protocol, data structures, and event-driven server architecture.

The project implements a single-threaded TCP server using non-blocking sockets and Linux epoll. It supports multiple client connections, incremental request parsing, pipelined commands, in-memory data structures, key expiration, eviction policies, and graceful shutdown.

> **Note:** This is an educational Redis-inspired implementation, not a drop-in replacement for Redis and not intended for production use.

## Features

- **TCP server:** Non-blocking sockets with Linux epoll-based I/O multiplexing.
- **Event-driven architecture:** Single-threaded event loop with sequential command execution.
- **Multiple clients:** Per-client read and write buffers.
- **Partial I/O handling:** Handles incomplete reads, partial writes, and non-blocking socket conditions.
- **Pipelining:** Processes multiple requests received in a single read buffer.
- **RESP protocol:** Incremental RESP2 parsing and response encoding.
- **Inline commands:** Basic inline command parsing.
- **In-memory storage:** String, Set, Sorted Set, Bloom Filter, and Count-Min Sketch.
- **Key expiration:** TTL and EXPIRE support with lazy expiration.
- **Eviction:** Random and approximate LRU policies.
- **Server information:** Keyspace statistics through INFO.
- **Graceful shutdown:** Stops accepting new connections and allows existing connections to finish within a configurable timeout.

## Architecture

```text
                    TCP Clients
                         |
                         v
               Non-blocking TCP Server
                         |
                         v
                  Linux epoll
                         |
                         v
                  Event Loop
                         |
              +----------+----------+
              |                     |
          Read Events           Write Events
              |                     |
              v                     v
       Client Read Buffer    Client Write Buffer
              |
              v
        RESP / Inline Parser
              |
              v
         Command Dispatcher
              |
              v
       In-memory Key-Value Store
              |
      +-------+-------+-------+---------+
      |       |       |       |         |
    String   Set   Sorted Set Bloom    CMS
```

### I/O Multiplexing

The server uses Linux `epoll` to monitor socket events.

The I/O multiplexer provides the following operations:

- `Monitor`: Register a file descriptor for read/write events.
- `Update`: Modify the events monitored for a file descriptor.
- `Remove`: Unregister a file descriptor.
- `Wait`: Wait for available I/O events.
- `Wakeup`: Wake up the event loop using Linux `eventfd`.

### Event Loop

The server processes client events sequentially in a single-threaded event loop.

Each client has:

- A read buffer for accumulating incoming data.
- A write buffer for outgoing responses.

The server handles incomplete requests, multiple requests in one read, partial writes, and non-blocking I/O conditions.

It does not use a worker pool or execute commands concurrently.

## Supported Commands

### String

| Command | Syntax |
|---|---|
| PING | `PING [message]` |
| ECHO | `ECHO message` |
| SET | `SET key value` |
| GET | `GET key` |

### Key Management

| Command | Syntax |
|---|---|
| DEL | `DEL key [key ...]` |
| EXISTS | `EXISTS key [key ...]` |
| EXPIRE | `EXPIRE key seconds` |
| TTL | `TTL key` |

TTL return values:

- `-2`: Key does not exist or has expired.
- `-1`: Key exists without an expiration.
- Non-negative integer: Remaining TTL in seconds.

### Set

| Command | Syntax |
|---|---|
| SADD | `SADD key member [member ...]` |
| SREM | `SREM key member [member ...]` |
| SISMEMBER | `SISMEMBER key member` |
| SMEMBERS | `SMEMBERS key` |

### Sorted Set

| Command | Syntax |
|---|---|
| ZADD | `ZADD key score member [score member ...]` |
| ZSCORE | `ZSCORE key member` |
| ZRANK | `ZRANK key member` |

Sorted Set uses a hash map for member-to-score lookup and a skip list to maintain ordering by score, with lexicographical member ordering as the tie-breaker.

### Count-Min Sketch

| Command | Syntax |
|---|---|
| CMS.INITBYPROB | `CMS.INITBYPROB key epsilon delta` |
| CMS.INITBYDIM | `CMS.INITBYDIM key width depth` |
| CMS.INCRBY | `CMS.INCRBY key item increment [item increment ...]` |
| CMS.QUERY | `CMS.QUERY key item [item ...]` |
| CMS.INFO | `CMS.INFO key` |

Count-Min Sketch estimates item frequencies using a two-dimensional counter array and multiple hash functions.

### Bloom Filter

| Command | Syntax |
|---|---|
| BF.RESERVE | `BF.RESERVE key errorRate capacity` |
| BF.ADD | `BF.ADD key item` |
| BF.EXISTS | `BF.EXISTS key item` |
| BF.MADD | `BF.MADD key item [item ...]` |
| BF.MEXISTS | `BF.MEXISTS key item [item ...]` |

`BF.ADD` automatically creates a Bloom Filter if the key does not exist, using:

- Default capacity: 100.
- Default false-positive rate: 1%.

Bloom Filters may return false positives but do not normally return false negatives. They do not support deletion.

### Server Information

| Command | Syntax |
|---|---|
| INFO | `INFO` |

`INFO` currently reports keyspace statistics:

- Total number of valid keys.
- Number of keys with expiration.
- Average remaining TTL in milliseconds.

## Data Structures

| Data Structure | Implementation |
|---|---|
| String | Go string values |
| Set | Hash map |
| Sorted Set | Hash map + Skip List |
| Bloom Filter | Bit array + double hashing |
| Count-Min Sketch | Two-dimensional counter array + MurmurHash3 |

### Skip List

The Sorted Set uses a skip list with a maximum level of 32.

- Search, insertion, and removal: Expected O(log N).
- Rank lookup: O(N) in the current implementation.
- Score lookup: Average O(1) through the hash map.

### Bloom Filter

The Bloom Filter calculates its bit-array size and number of hash functions from the requested capacity and false-positive rate.

It uses double hashing to derive multiple bit positions from two hash values.

### Count-Min Sketch

The Count-Min Sketch supports initialization by error parameters or explicit dimensions.

It uses MurmurHash3 and double hashing to derive counter positions. Counter increments use saturating arithmetic.

## Expiration and Eviction

### Key Expiration

Keys can be assigned an expiration time through EXPIRE.

Expiration is handled lazily:

- Expired keys are removed when accessed.
- Expired keys are purged during capacity checks and keyspace statistics collection.
- There is no periodic background expiration scheduler.

### Eviction Policies

The store has a configurable maximum of 10,000 keys.

Supported policies:

- **Random:** Selects a random key using reservoir sampling.
- **Approximate LRU:** Samples a configurable number of keys and removes the least recently accessed key among the sample.

Default configuration:

| Setting | Default |
|---|---|
| Maximum keys | 10,000 |
| Eviction policy | `random` |
| LRU sample count | 5 |

The LRU implementation is approximate rather than an exact global LRU.

## Graceful Shutdown

The server handles SIGINT and SIGTERM.

On shutdown:

1. The signal handler wakes up the event loop through `eventfd`.
2. The listening socket is removed from epoll and closed.
3. New client connections are no longer accepted.
4. Existing client connections are retained during the shutdown period.
5. The server exits when the tracked client connections have closed or the shutdown timeout expires.
6. Remaining connections are force-closed when the timeout is reached.

The current shutdown timeout is 10 seconds.

## Configuration

Configuration is defined in `internal/config/config.go`.

| Setting | Value |
|---|---|
| Protocol | TCP |
| Host | `localhost` |
| Port | `6379` |
| Max connections | `20000` (declared but not currently enforced) |
| Max epoll events per wait | `1024` |
| Maximum keys | `10000` |
| Eviction policy | `random` |
| Eviction samples | `5` |

The I/O multiplexing implementation currently targets Linux.

## Requirements

- Go 1.24.0 or later.
- Linux environment (or WSL2 with Ubuntu).
- `redis-cli` for manual testing.
- `redis-benchmark` for benchmarking.

## Build and Run

Clone the repository:

```bash
git clone https://github.com/kientthust-dev/redis-from-scratch.git
cd redis-from-scratch
```

Build:

```bash
go build -o redis_core .
```

Run:

```bash
./redis_core
```

The server listens on:

```text
localhost:6379
```

Connect using Redis CLI:

```bash
redis-cli -h 127.0.0.1 -p 6379
```

Example:

```text
127.0.0.1:6379> PING
PONG

127.0.0.1:6379> SET name redis
OK

127.0.0.1:6379> GET name
"redis"

127.0.0.1:6379> EXPIRE name 60
(integer) 1

127.0.0.1:6379> TTL name
(integer) 60
```

## Testing

Run all tests:

```bash
go test ./...
```

Run with the Go race detector:

```bash
go test -race ./...
```

## Benchmark

Benchmarks were run locally using `redis-benchmark` against this server on WSL2 Ubuntu.

### Environment

- WSL2 Ubuntu 24.04.4.
- Go 1.24.0.
- Local TCP connection to `127.0.0.1:6379`.
- 10 concurrent clients.
- Pipeline depth: 1 or 16.
- 100,000 requests per benchmark.
- Small default benchmark payloads.

### Results

| Command | Clients | Pipeline | Throughput (requests/sec) |
|---|---:|---:|---:|
| PING_INLINE | 10 | 1 | 145,772 |
| PING_MBULK | 10 | 1 | 125,313 |
| PING_INLINE | 10 | 16 | 1,190,476 |
| PING_MBULK | 10 | 16 | 1,123,595 |
| SET | 10 | 1 | 91,241 |
| GET | 10 | 1 | 163,666 |
| SET | 10 | 16 | 662,252 |
| GET | 10 | 16 | 724,638 |

Pipeline depth 16 significantly increased throughput in these local tests, while increasing average and tail latency.

These results are specific to the local WSL2 environment and benchmark workload. They should not be interpreted as production performance or as a direct comparison with Redis.

## Limitations

This project is intentionally scoped as an educational implementation.

It currently does not provide:

- RESP3 support.
- Full Redis protocol or command compatibility.
- Persistence (RDB/AOF).
- Replication or clustering.
- Pub/Sub.
- Transactions.
- Authentication or ACL.
- Background expiration scheduling.
- LFU eviction.
- Concurrent command execution or a worker pool.
- Production-grade resource management and operational guarantees.

## Technologies

- Go
- Linux system calls
- TCP sockets
- epoll
- eventfd
- RESP2
- Skip List
- Bloom Filter
- Count-Min Sketch
- MurmurHash3

## License

Add a license file if you intend to distribute or reuse this project under a specific open-source license.