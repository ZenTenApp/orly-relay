# IPC Sync Services Architecture

ORLY supports splitting sync functionality into independent gRPC IPC services for improved modularity, resource isolation, and horizontal scaling.

## Overview

The sync subsystem can run in two modes:

1. **Local mode** (default): All sync functionality runs in-process with the main relay
2. **gRPC mode**: Sync services run as separate processes communicating via gRPC

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              orly-launcher                                   │
│  (Process supervisor - manages lifecycle of all services)                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
           ┌──────────────────────────┼──────────────────────────┐
           │                          │                          │
           ▼                          ▼                          ▼
┌─────────────────────┐   ┌─────────────────────┐   ┌─────────────────────┐
│     orly-db         │   │     orly-acl        │   │   Sync Services     │
│   (gRPC :50051)     │   │   (gRPC :50052)     │   │   (gRPC :50061-64)  │
│                     │   │                     │   │                     │
│ - Event storage     │   │ - Access control    │   │ - Distributed sync  │
│ - Query execution   │   │ - Follow lists      │   │ - Cluster sync      │
│ - Index management  │   │ - Permissions       │   │ - Relay groups      │
└─────────────────────┘   └─────────────────────┘   │ - Negentropy/NIP-77 │
           │                          │             └─────────────────────┘
           │                          │                          │
           └──────────────────────────┼──────────────────────────┘
                                      │
                                      ▼
                          ┌─────────────────────┐
                          │       orly          │
                          │   (Main relay)      │
                          │                     │
                          │ - WebSocket server  │
                          │ - HTTP endpoints    │
                          │ - NIP handling      │
                          └─────────────────────┘
```

## Sync Services

### 1. orly-sync-distributed (Port 50061)

Serial-based peer-to-peer synchronization between relay instances.

**Purpose**: Keeps multiple relay instances in sync by exchanging events based on serial numbers (Lamport clocks).

**Key Features**:
- Periodic polling of peer relays
- NIP-11 relay info caching for peer identification
- NIP-98 authenticated sync requests
- Incremental sync using serial numbers

**gRPC API**:
```protobuf
service DistributedSyncService {
  rpc Ready(Empty) returns (ReadyResponse);
  rpc GetInfo(Empty) returns (SyncInfo);
  rpc GetCurrentSerial(CurrentRequest) returns (CurrentResponse);
  rpc GetEventIDs(EventIDsRequest) returns (EventIDsResponse);
  rpc HandleCurrentRequest(HTTPRequest) returns (HTTPResponse);
  rpc HandleEventIDsRequest(HTTPRequest) returns (HTTPResponse);
  rpc GetPeers(Empty) returns (PeersResponse);
  rpc UpdatePeers(UpdatePeersRequest) returns (Empty);
  rpc GetSyncStatus(Empty) returns (SyncStatusResponse);
  rpc GetPeerPubkey(PeerPubkeyRequest) returns (PeerPubkeyResponse);
  rpc IsAuthorizedPeer(AuthorizedPeerRequest) returns (AuthorizedPeerResponse);
  rpc NotifyNewEvent(NewEventNotification) returns (Empty);
}
```

**Environment Variables**:
| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_SYNC_DISTRIBUTED_LISTEN` | 127.0.0.1:50061 | gRPC listen address |
| `ORLY_SYNC_DISTRIBUTED_DB_TYPE` | grpc | Database backend |
| `ORLY_SYNC_DISTRIBUTED_DB_SERVER` | 127.0.0.1:50051 | Database server address |
| `ORLY_SYNC_DISTRIBUTED_PEERS` | | Comma-separated peer URLs |
| `ORLY_SYNC_DISTRIBUTED_INTERVAL` | 30s | Sync polling interval |

---

### 2. orly-sync-cluster (Port 50062)

Cluster replication with persistent state for high-availability deployments.

**Purpose**: Enables multiple relay instances to form a cluster with consistent event replication.

**Key Features**:
- Real-time event propagation to cluster members
- Cluster membership management via Kind 39108 events
- Persistent tracking of cluster member state
- Configurable privileged event propagation (DMs, gift wraps)

**gRPC API**:
```protobuf
service ClusterSyncService {
  rpc Ready(Empty) returns (ReadyResponse);
  rpc Start(Empty) returns (Empty);
  rpc Stop(Empty) returns (Empty);
  rpc HandleLatestSerial(HTTPRequest) returns (HTTPResponse);
  rpc HandleEventsRange(HTTPRequest) returns (HTTPResponse);
  rpc GetMembers(Empty) returns (MembersResponse);
  rpc UpdateMembership(UpdateMembershipRequest) returns (Empty);
  rpc HandleMembershipEvent(MembershipEventRequest) returns (Empty);
  rpc GetClusterStatus(Empty) returns (ClusterStatusResponse);
  rpc GetMemberStatus(MemberStatusRequest) returns (MemberStatusResponse);
  rpc GetLatestSerial(Empty) returns (LatestSerialResponse);
  rpc GetEventsInRange(EventsRangeRequest) returns (EventsRangeResponse);
}
```

**Environment Variables**:
| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_SYNC_CLUSTER_LISTEN` | 127.0.0.1:50062 | gRPC listen address |
| `ORLY_SYNC_CLUSTER_DB_TYPE` | grpc | Database backend |
| `ORLY_SYNC_CLUSTER_DB_SERVER` | 127.0.0.1:50051 | Database server address |
| `ORLY_CLUSTER_ADMINS` | | Authorized cluster admin pubkeys |
| `ORLY_CLUSTER_PROPAGATE_PRIVILEGED_EVENTS` | true | Replicate DMs/gift wraps |

---

### 3. orly-sync-relaygroup (Port 50063)

Relay group configuration discovery using Kind 39105 events.

**Purpose**: Manages relay group configurations that define which relays should sync together.

**Key Features**:
- Discovery of authoritative relay group configs
- Validation of relay group events
- Authorized publisher management
- Dynamic peer list updates

**gRPC API**:
```protobuf
service RelayGroupService {
  rpc Ready(Empty) returns (ReadyResponse);
  rpc FindAuthoritativeConfig(Empty) returns (RelayGroupConfigResponse);
  rpc GetRelays(Empty) returns (RelaysResponse);
  rpc IsAuthorizedPublisher(AuthorizedPublisherRequest) returns (AuthorizedPublisherResponse);
  rpc GetAuthorizedPubkeys(Empty) returns (AuthorizedPubkeysResponse);
  rpc ValidateRelayGroupEvent(ValidateEventRequest) returns (ValidateEventResponse);
  rpc HandleRelayGroupEvent(HandleEventRequest) returns (Empty);
}
```

**Environment Variables**:
| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_SYNC_RELAYGROUP_LISTEN` | 127.0.0.1:50063 | gRPC listen address |
| `ORLY_SYNC_RELAYGROUP_DB_TYPE` | grpc | Database backend |
| `ORLY_SYNC_RELAYGROUP_DB_SERVER` | 127.0.0.1:50051 | Database server address |
| `ORLY_RELAY_GROUP_ADMINS` | | Authorized relay group config publishers |

---

### 4. orly-sync-negentropy (Port 50064)

NIP-77 negentropy-based efficient set reconciliation.

**Purpose**: Provides efficient set reconciliation for both relay-to-relay sync and client-facing NIP-77 WebSocket protocol.

**Key Features**:
- NIP-77 client WebSocket support (NEG-OPEN, NEG-MSG, NEG-CLOSE)
- Relay-to-relay negentropy sync
- Session management for concurrent clients
- Configurable frame size and ID truncation

**gRPC API**:
```protobuf
service NegentropyService {
  rpc Ready(Empty) returns (ReadyResponse);
  rpc Start(Empty) returns (Empty);
  rpc Stop(Empty) returns (Empty);

  // Client-facing NIP-77
  rpc HandleNegOpen(NegOpenRequest) returns (NegOpenResponse);
  rpc HandleNegMsg(NegMsgRequest) returns (NegMsgResponse);
  rpc HandleNegClose(NegCloseRequest) returns (Empty);

  // Relay-to-relay sync
  rpc SyncWithPeer(SyncPeerRequest) returns (stream SyncProgress);
  rpc GetSyncStatus(Empty) returns (SyncStatusResponse);
  rpc GetPeers(Empty) returns (PeersResponse);
  rpc AddPeer(AddPeerRequest) returns (Empty);
  rpc RemovePeer(RemovePeerRequest) returns (Empty);
  rpc TriggerSync(TriggerSyncRequest) returns (Empty);
  rpc GetPeerSyncState(PeerSyncStateRequest) returns (PeerSyncStateResponse);

  // Session management
  rpc ListSessions(Empty) returns (ListSessionsResponse);
  rpc CloseSession(CloseSessionRequest) returns (Empty);
}
```

**Environment Variables**:
| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_SYNC_NEGENTROPY_LISTEN` | 127.0.0.1:50064 | gRPC listen address |
| `ORLY_SYNC_NEGENTROPY_DB_TYPE` | grpc | Database backend |
| `ORLY_SYNC_NEGENTROPY_DB_SERVER` | 127.0.0.1:50051 | Database server address |
| `ORLY_SYNC_NEGENTROPY_PEERS` | | Comma-separated peer WebSocket URLs |
| `ORLY_SYNC_NEGENTROPY_INTERVAL` | 60s | Background sync interval |
| `ORLY_SYNC_NEGENTROPY_FRAME_SIZE` | 4096 | Negentropy frame size |
| `ORLY_SYNC_NEGENTROPY_ID_SIZE` | 16 | ID truncation size |
| `ORLY_SYNC_NEGENTROPY_SESSION_TIMEOUT` | 5m | Client session timeout |

---

## Launcher Configuration

The `orly-launcher` manages all services. Enable sync services with these environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_LAUNCHER_SYNC_DISTRIBUTED_ENABLED` | false | Enable distributed sync |
| `ORLY_LAUNCHER_SYNC_DISTRIBUTED_BINARY` | orly-sync-distributed | Binary path |
| `ORLY_LAUNCHER_SYNC_DISTRIBUTED_LISTEN` | 127.0.0.1:50061 | Listen address |
| `ORLY_LAUNCHER_SYNC_CLUSTER_ENABLED` | false | Enable cluster sync |
| `ORLY_LAUNCHER_SYNC_CLUSTER_BINARY` | orly-sync-cluster | Binary path |
| `ORLY_LAUNCHER_SYNC_CLUSTER_LISTEN` | 127.0.0.1:50062 | Listen address |
| `ORLY_LAUNCHER_SYNC_RELAYGROUP_ENABLED` | false | Enable relay group |
| `ORLY_LAUNCHER_SYNC_RELAYGROUP_BINARY` | orly-sync-relaygroup | Binary path |
| `ORLY_LAUNCHER_SYNC_RELAYGROUP_LISTEN` | 127.0.0.1:50063 | Listen address |
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED` | false | Enable negentropy |
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_BINARY` | orly-sync-negentropy | Binary path |
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN` | 127.0.0.1:50064 | Listen address |
| `ORLY_LAUNCHER_SYNC_READY_TIMEOUT` | 30s | Sync service startup timeout |

---

## Relay Configuration

When sync services are enabled, configure the relay to connect:

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_SYNC_TYPE` | local | Sync backend: local or grpc |
| `ORLY_GRPC_SYNC_DISTRIBUTED` | | Distributed sync server address |
| `ORLY_GRPC_SYNC_CLUSTER` | | Cluster sync server address |
| `ORLY_GRPC_SYNC_RELAYGROUP` | | Relay group server address |
| `ORLY_GRPC_SYNC_NEGENTROPY` | | Negentropy server address |
| `ORLY_GRPC_SYNC_TIMEOUT` | 10s | gRPC connection timeout |
| `ORLY_NEGENTROPY_ENABLED` | false | Enable NIP-77 WebSocket support |

---

## Startup Order

The launcher starts services in dependency order:

1. **Database** (orly-db) - Must be ready first
2. **ACL** (orly-acl) - Depends on database
3. **Sync Services** (parallel) - All depend on database only
4. **Relay** (orly) - Depends on all above

Shutdown happens in reverse order.

---

## Example: Enabling Negentropy

To enable NIP-77 negentropy support on an existing deployment:

```bash
# In systemd service or environment
Environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true
Environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_BINARY=/path/to/orly-sync-negentropy
Environment=ORLY_NEGENTROPY_ENABLED=true
```

Build the binary:
```bash
CGO_ENABLED=0 go build -o orly-sync-negentropy ./cmd/orly-sync-negentropy
```

---

## Proto Definitions

Proto files are located in:
- `proto/orlysync/common/v1/types.proto` - Shared types
- `proto/orlysync/distributed/v1/service.proto` - Distributed sync
- `proto/orlysync/cluster/v1/service.proto` - Cluster sync
- `proto/orlysync/relaygroup/v1/service.proto` - Relay group
- `proto/orlysync/negentropy/v1/service.proto` - Negentropy

Generate Go code with:
```bash
buf generate
```

---

## NIP-77 Client Protocol

The negentropy service implements NIP-77 for efficient client synchronization:

### Client Messages
- `["NEG-OPEN", subscription_id, filter, initial_message?]` - Start reconciliation
- `["NEG-MSG", subscription_id, message]` - Continue reconciliation
- `["NEG-CLOSE", subscription_id]` - End session

### Server Responses
- `["NEG-MSG", subscription_id, message]` - Reconciliation response
- `["NEG-ERR", subscription_id, reason]` - Error response

The relay automatically routes these messages to the negentropy service when `ORLY_NEGENTROPY_ENABLED=true`.
