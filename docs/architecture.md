# HTTP Proxy Architecture

## System Context Diagram
```mermaid
graph TD
    Client[Browser / Client App]
    Proxy[ZenFlow Proxy Server]
    Internet[Internet / Target Web Servers]

    Client -- HTTP(S) Requests --> Proxy
    Proxy -- Forward/Tunnel --> Internet
    Internet -- HTTP Responses --> Proxy
    Proxy -- Return Responses --> Client
```

## Request Flow
```mermaid
sequenceDiagram
    participant Client
    participant Proxy as Proxy Server
    participant Cache as LRU Cache (Phase 2)
    participant Auth as Auth & Rate Limit
    participant Filter as Dual-Filter (Phase 3)
    participant Target as Target Web Server

    Client->>Proxy: Send HTTP Request
    Proxy->>Auth: Check Basic Auth & Rate Limiter
    alt Exceeds Limit or Auth Failed
        Auth-->>Proxy: Reject (429 or 407)
        Proxy-->>Client: Error Response
    else Valid Client
        Proxy->>Filter: Check Domain (Ads + Malware)
        alt is Blocked Domain
            Filter-->>Proxy: Block
            Proxy-->>Client: 403 Forbidden
        else is Clean Domain
            alt is Static Asset GET
                Proxy->>Cache: Check Cache
                alt Cache Hit
                    Cache-->>Proxy: Cached Response
                    Proxy-->>Client: Response (X-Cache: HIT)
                else Cache Miss
                    Proxy->>Target: Forward Request
                    Target-->>Proxy: HTTP Response
                    Proxy->>Cache: Store if small enough
                    Proxy-->>Client: Response (X-Cache: MISS)
                end
            else Dynamic or CONNECT
                Proxy->>Target: Forward/Tunnel Request
                Target-->>Proxy: HTTP Response/TCP Stream
                Proxy-->>Client: Response/TCP Stream
            end
        end
    end
```

## Internal Component Architecture
```mermaid
graph LR
    subgraph cmd [cmd/proxy/]
        main[main.go - App Entry Point]
    end

    subgraph pkg [pkg/]
        proxy[proxy/ - HTTP Handlers & HTTPS Tunnel]
        filter[filter/ - Blocklists]
        logger[logger/ - Channel Logger (Phase 4)]
        ratelimit[ratelimit/ - Token Bucket]
        auth[auth/ - Basic Auth]
        tui[tui/ - Bubble Tea UI (Phase 4)]
        config[config/ - Auto Refresh & File Watcher]
    end

    main --> proxy
    main --> tui
    main --> config
    proxy --> filter
    proxy --> logger
    proxy --> ratelimit
    proxy --> auth
    tui --> logger
```
