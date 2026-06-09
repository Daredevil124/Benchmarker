# IICPC Distributed Benchmarking Engine

![Build Status](https://img.shields.io/badge/build-passing-brightgreen)
![License](https://img.shields.io/badge/license-MIT-blue)
![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)
![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?logo=docker)

A highly concurrent, resilient, and decoupled **Distributed Benchmarking and Hosting Platform** built for the **IICPC Summer Hackathon 2026**. This system is designed to securely evaluate and stress-test contestant-submitted trading infrastructure under simulated peak market volatility.

## 🚀 Key Features

- **Secure Polyglot Sandboxing:** Safely compiles and executes contestant code (C++, Python, Node.js, Java) within strictly isolated, heavily restricted Docker-out-of-Docker (DooD) environments.
- **Distributed Bot Fleet (Load Generator):** Unleashes thousands of concurrent Goroutines across distributed worker nodes to simulate high-velocity REST traffic and bombard target endpoints.
- **Granular Telemetry Ingestion:** Accurately measures low-latency interactions, capturing real-time P50, P90, P99 latencies, and Transactions Per Second (TPS).
- **Real-Time Dynamic Leaderboard:** A sleek React frontend that streams live metrics and contest standings via WebSockets, dynamically ranking submissions based on a composite score of speed, stability, and algorithmic correctness.
- **Linearly Scalable Architecture:** Leverages Redis Pub/Sub for lightweight inter-service communication, allowing the grading engine and bot fleet to scale infinitely across a Docker Swarm.

---

## 🏗️ Architecture Overview

The platform adopts a decoupled microservices paradigm to ensure massive horizontal scalability and fault tolerance:

1. **Master Node (API Gateway):** 
   - Handles multi-part code uploads and authenticates submissions.
   - Pushes tasks to the `grading_queue` and broadcasts load-testing targets via the `attack_commands` Pub/Sub channel.
   - Streams aggregated telemetry to the UI over WebSockets.
2. **Worker Nodes (Bot Fleet & Grader):**
   - The execution workhorses. They consume tasks, spawn strictly isolated Docker containers, and validate output correctness.
   - Subscribes to attack commands to unleash local concurrency (bot fleets) and publishes calculated P50/P90/P99 latencies back to the Master.
3. **Redis (Message Broker & Datastore):**
   - Acts as the central nervous system for inter-service communication (Pub/Sub & Queues) and maintains in-memory leaderboard standings to prevent disk I/O bottlenecks.

> 📄 For an in-depth look at our design decisions, please see our [Architecture Blueprint](Architecture_Blueprint.md).

---

## ⚙️ Quick Start Guide

### Prerequisites
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) installed.

### 1. Clone the Repository
```bash
git clone https://github.com/your-username/Benchmarker.git
cd Benchmarker
```

### 2. Launch the Cluster
Using Infrastructure as Code (IaC), you can spin up the entire multi-tier architecture (Master, Workers, Redis, and Frontend) with a single command:
```bash
docker compose up -d --build
```
*Note: You can scale the bot fleet and grading engine effortlessly:*
```bash
docker compose up -d --scale worker=5
```

### 3. Access the Platform
- **React Leaderboard UI:** Navigate to `http://localhost:8080`
- **Master API Endpoint:** Listening on `http://localhost:9000`

---

## 🛡️ Security & Isolation

Evaluating arbitrary, untrusted contestant code requires extreme security measures. Our Sandboxing Engine isolates executions using:
- **CPU Pinning & Memory Limits:** `--cpus=0.5` and `--memory=256m` prevent resource exhaustion.
- **Complete Network Isolation:** `--network=none` prevents data exfiltration.
- **Privilege Dropping:** `--cap-drop=ALL` revokes Linux kernel capabilities.
- **Process Limits:** `--pids-limit=64` to prevent fork bombs.

---

## 🛠️ Tech Stack
- **Backend Core:** Golang
- **Frontend Client:** React, Vite
- **Message Broker & Datastore:** Redis
- **Containerization & IaC:** Docker, Docker Compose

---
*Built with hardcore engineering excellence for the IICPC Summer Hackathon 2026.*