# Distributed Shared Memory - TEAM 6

## Team Members
1. Manuj Garg - 2021101047 
2. Pranav Gupta - 2021101095 
3. Gowlapalli Rohit - 2021101113 

# Overview
```
A fault-tolerant distributed key-value store with master-slave replication and automatic failover capabilities.
```

## Features

- **Distributed Architecture**: Primary master with backup and multiple slave nodes
- **Fault Tolerance**: Automatic failover to backup master
- **Data Replication**: Quorum-based writes (ceil(n/2) slaves)
- **Consistency**: Majority voting for reads when needed
- **Durability**: Write-ahead logging on master
- **Client Resilience**: Automatic reconnection to backup on failure

## System Components

1. **Master Server**: Primary coordinator (port 12345)
2. **Backup Master**: Hot standby (port 12346)
3. **Slave Nodes**: Data storage replicas
4. **Client**: Command-line interface for key-value operations

## Prerequisites

- Go 1.16+ installed
- Basic network connectivity between nodes

## Building

Build all components:
```bash
go build -o master/master master/main.go
go build -o backup_master/backup_master backup_master/main.go
go build -o slave/slave slave/main.go
go build -o client/client client/main.go
```

## Running the System

### 1. Start the Master Server
```bash
./master/master
```

### 2. Start the Backup Master (in a separate terminal)
```bash
./backup_master/backup_master
```

### 3. Start Slave Nodes (in separate terminals, as many as needed)
```bash
./slave/slave
```

### 4. Run Client Applications (in separate terminals, as many as needed)
```bash
./client/client
```

## Usage Instructions

### Client Operations

When running the client, you'll see a prompt:
```
[PRIMARY] Enter the Operation you would like to perform (READ/WRITE/EXIT):
```

Available commands:
- **READ**: Retrieve a value by key
  ```
  READ <key>
  ```
- **WRITE**: Store a key-value pair
  ```
  WRITE <key> <value>
  ```
- **EXIT**: Quit the client

### Example Session

1. Write a value:
   ```
   [PRIMARY] Enter the Operation you would like to perform (READ/WRITE/EXIT): WRITE
   Enter the key you want to perform the operation on: foo
   Enter the value corresponding to the key: bar
   ```

2. Read the value:
   ```
   [PRIMARY] Enter the Operation you would like to perform (READ/WRITE/EXIT): READ
   Enter the key you want to perform the operation on: foo
   ```

## Fault Tolerance Demonstration

1. With the system running, kill the primary master process
2. Observe:
   - Clients automatically reconnect to the backup master
   - Slave nodes reconnect to the backup master
   - System continues operating with "[BACKUP]" indicator in client prompts
3. Restart the primary master to see failback behavior

## Configuration

Default ports (can be changed in code):
- Master: 12345
- Backup Master: 12346

## Monitoring

- Master and backup master log operations to console
- Slave nodes log received commands
- Client shows which server it's connected to (PRIMARY/BACKUP)

## Cleanup

To stop the system:
1. Stop all client processes (Ctrl+C)
2. Stop slave processes (Ctrl+C)
3. Stop backup master (Ctrl+C)
4. Stop primary master (Ctrl+C)

## Troubleshooting

1. **Port conflicts**: Ensure no other services are using ports 12345/12346
2. **Connection issues**: Verify all components can reach each other over network
3. **Log files**: Check `kv_store.log` in master directory for operation history


### Command to perform scaled_testing
```bash
go run test_harness/main.go -slaves 5 -clients 18
```