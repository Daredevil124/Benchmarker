# Architecture Blueprint: Distributed Benchmarking & Hosting Platform

## 1. System Overview

Our platform is a highly concurrent, resilient, and decoupled distributed system designed to securely host contestant-submitted trading infrastructure and evaluate it under massive, simulated market volatility.

The architecture strictly follows a microservices paradigm, leveraging **Golang** for high-performance concurrent processing, **Redis** for fast in-memory data storage and inter-service messaging, and **Docker** for strict runtime isolation.

## 2. Microservices Architecture

The system is decoupled into the following distinct components:

### A. API Gateway & Master Node (`benchmarker-master`)
- **Role:** Acts as the central ingress point for all contestant traffic and administrator commands.
- **Responsibilities:**
  - Handles multipart code uploads (`/api/upload`) and authenticates submissions.
  - Pushes grading tasks to the distributed queue.
  - Broadcasts load-testing commands to the bot fleet.
  - Aggregates telemetry data and streams it to the frontend via WebSockets (`/api/stream`).

### B. Distributed Worker Nodes (`benchmarker-worker`)
- **Role:** The execution workhorses of the platform. Designed to scale horizontally to thousands of instances.
- **Responsibilities:**
  - Consumes grading tasks from the queue and spins up isolated sandboxes (Docker-out-of-Docker).
  - Listens for attack commands and spawns high-concurrency "Bot Fleets" (Goroutines) to bombard contestant endpoints.
  - Acts as the **Telemetry Ingester**, calculating granular metrics (p50, p90, p99 latencies, and TPS) locally before publishing back to the Master node to prevent bottlenecking.

### C. Real-Time Leaderboard (React Frontend)
- **Role:** The client-facing analytics interface.
- **Responsibilities:**
  - Connects to the Master node via WebSockets to consume the `telemetry_stream`.
  - Dynamically ranks contestants using a composite score based on algorithmic correctness, speed (latency), and stability (TPS efficiency).

## 3. Inter-Service Communication Protocols

To guarantee low latency and decouple the event producers from the consumers, we chose **Redis Pub/Sub** and **Redis Lists** as our primary communication backbone, acting as a lightweight alternative to Kafka/gRPC.

- **Asynchronous Task Queue (`grading_queue`):** A Redis List is used to buffer code submissions. Workers consume this queue using `BLPOP`, ensuring fair distribution and resilience against sudden traffic spikes.
- **Command Broadcasting (`attack_commands`):** A Redis Pub/Sub channel. When the Master node triggers a load test, it publishes the target coordinates. ALL active worker nodes subscribe to this channel and immediately unleash their local bot fleets simultaneously.
- **Telemetry Streaming (`grading_results`, `telemetry_stream`):** Workers publish their calculated p50/p90/p99 and TPS metrics back to the Master node via Pub/Sub. The Master node aggregates this and pushes it over a standard **WebSocket** connection to the React frontend.

## 4. Data Stores

- **Redis (In-Memory Datastore):** We completely forgo traditional relational databases (like PostgreSQL) to avoid disk I/O bottlenecks during peak traffic. Redis Hash Maps (`scores:team`) and Sorted Sets (`leaderboard`) are used to maintain real-time contest standings.

## 5. Security & Isolation Strategies

Evaluating arbitrary, untrusted contestant code requires extreme security measures. We implemented a strict Sandboxing Engine using Docker-out-of-Docker (DooD):

- **Ephemeral Workspaces:** Every submission is written to a unique, temporary host directory that is volume-mounted into the container as read-only (except for necessary output paths).
- **Strict Resource Constraints:** The isolated grading container (`benchmarker-sandbox`) is executed with:
  - `--memory=256m`: Prevents Memory Exhaustion (OOM) attacks.
  - `--cpus="0.5"`: CPU pinning ensures fair compute allocation across all contestants.
  - `--pids-limit=64`: Prevents Fork Bombs.
- **Network Isolation:** Executed with `--network="none"`, completely disabling external internet access to prevent data exfiltration or reverse shells.
- **Privilege Dropping:** Run with `--cap-drop=ALL` to revoke all Linux kernel capabilities.

## 6. Infrastructure as Code (IaC) & Scalability

The entire infrastructure is codified in a single `docker-compose.yml` stack.

- **Horizontal Scalability:** The Bot Fleet and Grading Engine can be scaled infinitely across a swarm by simply increasing the worker replica count (`docker compose up --scale worker=50`).
- **Resiliency:** Services are configured with retry loops to handle transient connection drops (e.g., Redis booting slower than the Go binaries). 
