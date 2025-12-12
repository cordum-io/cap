# Getting Started with CAP

This guide will walk you through setting up your environment, installing the necessary tools, and running a simple CAP application.

## Prerequisites

Before you begin, you will need to have the following installed:

*   **Go:** [https://golang.org/](https://golang.org/)
*   **Python:** [https://www.python.org/](https://www.python.org/)
*   **Node.js:** [https://nodejs.org/](https://nodejs.org/)
*   **C++ Compiler:** (e.g., g++, clang++)
*   **protoc:** The Protocol Buffers compiler. [Installation Guide](https://grpc.io/docs/protoc-installation/)
*   **NATS Server:** [https://nats.io/](https://nats.io/)

## Installation

1.  **Clone the CAP repository:**

    ```bash
    git clone https://github.com/coretexos/cap.git
    cd cap
    ```

2.  **Generate the Protobuf stubs:**

    The CAP protocol is defined using Protocol Buffers. To use the SDKs, you first need to generate the language-specific stubs from the `.proto` files.

    ```bash
    ./tools/make_protos.sh
    ```

3.  **Install SDK dependencies:**

    *   **Go:**

        ```bash
        cd sdk/go
        go mod tidy
        ```

    *   **Python:**

        ```bash
        cd sdk/python
        python3 -m venv .venv
        source .venv/bin/activate
        pip install -r requirements.txt
        ```
        *(Note: You will need to create a `requirements.txt` file)*

    *   **Node.js:**

        ```bash
        cd sdk/node
        npm install
        ```

## Running the Examples

The `examples` directory contains several examples that demonstrate how to use the CAP SDKs.

### Simple Echo

The `simple-echo` example demonstrates a simple worker that echoes back the job it receives.

1.  **Start a NATS server:**

    ```bash
    nats-server
    ```

2.  **Run the Go worker:**

    *(You will need to create a `main.go` file for the worker under `examples/simple-echo/go-worker`)*

    ```bash
    cd examples/simple-echo/go-worker
    go run .
    ```

3.  **Submit a job using the Go client:**

    *(You will need to create a `main.go` file for the client under `examples/simple-echo/go-client`)*

    ```bash
    cd examples/simple-echo/go-client
    go run .
    ```

You should see the worker receive the job and the client receive the result.
