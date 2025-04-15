# Distributed Shared Memory System

## Overview

This **Distributed Shared Memory (DSM)** system simulates READ and WRITE operations between multiple clients and slaves, coordinated by a central master server. Built using Python and socket programming, the system runs on a single machine and emphasizes:

- **Fault tolerance** using ACK bits from slaves.
- **Data consistency** through controlled distribution of WRITE operations.

## Features

- **Multiple Clients and Slaves**: Supports concurrent interactions with multiple clients and slave nodes.
- **Fault Tolerance**: Uses ACK bits from slaves to confirm successful writes, enabling failure detection.
- **Consistency and Efficiency**: WRITE operations are distributed to a subset of slaves to balance load and maintain up-to-date data access.

## Project Structure

- `client/main.go` — Manages client-side READ and WRITE operations.
- `master/main.go` — Orchestrates communication between clients and slaves, handles requests, and maintains system state.
- `slave/main.go` — Stores and processes data, and responds to commands from the master.
- `test_harness/main.go` — Simulates load on the system, generating metrics for performance analysis.

---

## File Details

### `client/main.go`

#### Functionality

Handles client interaction with the system, enabling users to perform READ and WRITE operations.

#### Workflow

- Connects to the master via socket.
- Sends user-input commands to the master and displays received responses.

---

### `master/main.go`

#### Functionality

Acts as the central coordinator in the DSM system. Responsibilities include managing connections, processing client requests, ensuring data consistency, and monitoring slave health.

#### Workflow

- **Connection Management**
  - Listens for incoming client and slave connections.
  - Identifies connection type and registers the participant.

- **Handling WRITE Operations**
  - Upon a WRITE request, selects a subset of slaves using a hash-based strategy.
  - Sends the WRITE command to selected slaves.
  - Waits for ACKs from slaves. If any slave fails to acknowledge within a timeout, it is removed from the active list.

- **Handling READ Operations**
  - Determines which slaves have the requested key.
  - Sends a READ request to retrieve the most recent value.
  - If a slave doesn't respond, retries with others.
  - If the key isn't found, returns `"NOT_FOUND"` to the client.

- **Backup Master Support**
  - On primary master failure, slaves and clients connect to the backup.
  - WRITE operations continue as normal.
  - READ operations are processed using the `key_to_slaves` mapping if available.
  - If the mapping is unavailable, the backup sends requests to all slaves, collects responses, identifies the majority value, and updates its internal mapping.

#### Consistency & Fault Tolerance

- **Consistency**
  - The master tracks which slaves store each key, ensuring READ operations return the latest value.
- **Fault Tolerance**
  - ACK bits allow the master to detect and isolate non-responsive slaves, ensuring continued system operation even during partial failures.

---

### `slave/main.go`

#### Functionality

Represents a storage node that processes commands from the master and stores key-value data.

#### Workflow

- Registers itself with the master upon startup.
- **WRITE Operation**: Stores data and sends an ACK back to the master.
- **READ Operation**: Returns the requested value to the master.

Here’s a detailed **report addition** for your `test_harness` under the **Scaled Testing** section, focusing on what the code accomplishes, how it works, and what insights it enables:

---

### `test_harness/main.go`

To evaluate the scalability and performance characteristics of the distributed key-value system, we developed a custom **test harness** in Go. This harness is designed to simulate a real-world load by spawning multiple clients and slave servers, coordinating their activities, and collecting detailed runtime metrics for analysis.

#### **Key Features of the Test Harness**

1. **Configurable Setup**
   - The number of **clients** and **slave servers** can be controlled via command-line flags (`-clients`, `-slaves`).
   - Each client creates a pool of TCP connections to the master node, simulating concurrent interactions.

2. **Dynamic Load Generation**
   - Clients randomly perform **read** (GET) and **write** (PUT) operations on a uniform keyspace (`key0` to `key999`).
   - The **write percentage** (default 30%) controls the read-write mix.
   - Each operation is rate-limited using a small sleep to avoid overloading the system unrealistically.

3. **Metrics Collection**
   - The harness continuously collects the following per-second metrics:
     - Total and successful requests
     - Error count
     - Average read and write latencies (in μs)
     - Read/write distribution
     - Throughput (requests per second)
   - These metrics are logged into both CSV and JSON formats for offline analysis.

4. **Warm-up Phase**
   - A warm-up period (`5s`) allows the system to reach a steady state before actual measurements begin.

5. **Graceful Shutdown and Interrupt Handling**
   - The harness captures interrupts (Ctrl+C), allowing all clients and slave processes to terminate cleanly.

6. **Data Visualization**
   - After the test run, a Python script (`visualize.py`) is automatically invoked to generate plots like:
     - Throughput vs. Clients
     - Latency vs. Clients
     - Error Rate vs. Clients
     - Read/Write Operation Breakdown


# Scaled Testing

### **Metrics by Slave Count**

#### **Throughput (Requests per Second)**
- **Description**: The number of requests the system can process per second.
- **What it means**: Throughput is a key indicator of system performance. Increasing throughput with additional slaves shows the system’s ability to handle more load.

![Throughput](results/plots_by_slave_count/throughput_rps_vs_slave_count.png)

#### **Average Read Latency**
- **Description**: The average time taken to complete a read operation.
- **What it means**: Lower latency is desired for quick retrieval of data. As slave count increases, the read latency should ideally stay constant or decrease.

![Average Read Latency](results/plots_by_slave_count/read_latency_us_vs_slave_count.png)

#### **Average Write Latency**
- **Description**: The average time taken to complete a write operation.
- **What it means**: Like read latency, lower write latency indicates better performance. If this metric rises as slaves are added, it may indicate contention or resource saturation.

![Average Write Latency](results/plots_by_slave_count/write_latency_us_vs_slave_count.png)


### **Metrics by Client Count**

#### **Throughput (Requests per Second)**
- **Description**: The number of requests processed per second by the system.
- **What it means**: A high throughput indicates a high-performing system, while a plateau or drop suggests that the system is unable to scale efficiently.

![Throughput](results/plots_by_client_count/throughput_rps_vs_client_count.png)

#### **Average Read Latency**
- **Description**: The average time taken for read operations.
- **What it means**: Increased read latency as client count rises typically suggests that the system is becoming slower as more clients access the data simultaneously.

![Average Read Latency](results/plots_by_client_count/read_latency_us_vs_client_count.png)

#### **Average Write Latency**
- **Description**: The average time taken for write operations.
- **What it means**: Write latency should ideally remain constant or improve with more clients. A rise in write latency with an increasing number of clients could indicate that the system is under heavy load.

![Average Write Latency](results/plots_by_client_count/write_latency_us_vs_client_count.png)
