# Explore Service

The Explore Service is a gRPC-based microservice designed to handle a subset of the features needed to create a dating app. Currently, the service is focused on handling decisions (like/pass) between users in an efficient way, assuming tables will grow over time and queries must be optimized. The project is built using Golang, Docker and MySQL.

## Key Features

- **User Decision Management**: Records and updates user decisions (like/pass) on other users with support for decision overwrites
- **Mutual Like Detection**: Automatically detects and reports when two users have mutually liked each other
- **Efficient Querying**: Provides paginated lists of users who liked a recipient, with support for filtering out already-matched users
- **Performance Optimizations**: Uses database indexes, cached like statistics, and cursor-based pagination to handle large-scale data efficiently
- **Atomic Operations**: Ensures data consistency through database transactions when recording decisions and updating statistics

## Requirements
- Go 1.24+
- Docker
- protoc (protobuf compiler)

## Layered Architecture

```
┌─────────────────────────────────────┐
│   explore-service.go (Handlers)     │  ← gRPC protocol layer
│   - Converts protobuf <> domain     │
│   - Calls business logic            │
│   - Handles gRPC errors             │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│   explore-business.go (Business)    │  ← Business logic layer
│   - Business rules                  │
│   - Orchestrates operations         │
│   - Uses repository for data access │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│   DB (MySQL)                        │  ← Data access layer
│   - Database queries                │
│   - Transaction management          │
└─────────────────────────────────────┘
```

## gRPC Endpoints
- ListLikedYou: List all users who liked the recipient.
- ListNewLikedYou: List all users who liked the recipient excluding those who have been liked in return.
- CountLikedYou: Count the number of users who liked the recipient.
- PutDecision: Record the decision of the actor to like or pass the recipient, then returns if a mutual like is detected.

## Assumptions
- Decisions can be overwritten and we do not need logs of their previous state in the DB.
- The decision table will grow considerably over time, thus we must avoid full scans over the tables and we must implement pagination in an efficient way.

## Optimizations
- Create indexes to avoid full scans operations over DB tables
- Create a like_stats table to keep track of total likes per user, avoiding COUNT() statements
- Implement cursor-based pagination
- Implement efficient queries avoiding CTE

## How to test it

TLDR: To run the server and test it using a client script

1. Open a terminal in the backend-interview-task folder, and run these commands one by one. The server will be set, listening to port 9001.
    ```bash
    make deps
    make server
    make logs-server
    ```
2. Once the server set up has finished, open a new terminal to run a client script in test-client.go:
    ```bash
    go run .\test-client\test-client.go
    ```
3. This will trigger calls to the server, displaying logs of those requests and responses. The calls include: checking the current like count of users, sending new decisions to the server and displaying the users who like a specific user_id.


More useful commands to manage the project DB and containers

1. **First Time Setup**:
    ```bash
    make deps      # Install dependencies and generate protobuf files
    make test      # Verify everything works
    ```

2. **Run Tests** (no Docker needed):
    ```bash
    make test      # Fast iteration, uses mocks
    ```

3. **Run Server Locally** (for quick development):
    ```bash
    make up        # Start MySQL docker container
    make run       # Run server locally (without docker)
    ```

4. **Run Server in Container**:
    ```bash
    make server    # Starts MySQL and server containers
    make logs-server  # Watch server logs
    ```

5. **Test with Client**:
    ```bash
    go run ./test-client/test-client.go  # Runs on your machine, connects to container
    ```

6. **Cleanup**:
    ```bash
    make down      # Stop everything (keeps data)
    make reset     # Stop everything and delete data
    ```

7. **Cleanup with Volumes**:
    ```bash
    make down      # Stop everything (keeps data)
    make clean     # Delete docker volumes
    ```



## Future work
- Implement integration test for Client - Server - DB layers, using dockertest for example.
- Fix env variables handling with external libraries
- Evaluate cache usage for common queries
- Add geo-location data to the users table, then create DB partitions based in regions, if business logic allows it
- Add a time window to our queries. Old decisions may not be valid after a year for example.
