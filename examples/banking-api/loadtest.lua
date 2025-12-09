-- Load test script for Banking API using wrk
-- This script tests the cache chain performance by:
-- 1. Creating transactions
-- 2. Reading transactions (testing cache hits)

-- Counter for tracking requests
local counter = 0
local thread_counter = 0

-- Store created transaction IDs for reading (shared across threads)
local transaction_ids = {}

-- Initialize per-thread state
function setup(thread)
    thread:set("id", thread_counter)
    thread_counter = thread_counter + 1
end

-- Generate request
function request()
    counter = counter + 1
    
    -- 20% creates, 80% reads (to test cache efficiency)
    if counter % 5 == 0 or #transaction_ids < 10 then
        return create_transaction()
    else
        return get_transaction()
    end
end

-- Create a new transaction
function create_transaction()
    local account_id = "ACC-" .. math.random(1, 100)
    local amount = math.random(10, 1000)
    
    local body = string.format([[{
        "account_id": "%s",
        "type": "debit",
        "amount": %.2f,
        "currency": "USD",
        "description": "Load test transaction"
    }]], account_id, amount)
    
    return wrk.format("POST", "/transactions", {
        ["Content-Type"] = "application/json"
    }, body)
end

-- Get an existing transaction
function get_transaction()
    -- Use stored transaction IDs to test real cache hits
    if #transaction_ids > 0 then
        local idx = math.random(1, #transaction_ids)
        local tx_id = transaction_ids[idx]
        
        return wrk.format("GET", "/transactions/" .. tx_id, {
            ["Content-Type"] = "application/json"
        })
    else
        -- Fallback: create if we don't have IDs yet
        return create_transaction()
    end
end

-- Process response and extract transaction IDs
function response(status, headers, body)
    -- Store transaction ID from successful creates using pattern matching
    if status == 201 and body then
        -- Extract ID from JSON: "id":"uuid-here"
        local id = body:match('"id"%s*:%s*"([^"]+)"')
        if id then
            table.insert(transaction_ids, id)
            -- Keep only last 1000 IDs to avoid memory issues
            if #transaction_ids > 1000 then
                table.remove(transaction_ids, 1)
            end
        end
    end
    
    if status ~= 200 and status ~= 201 and status ~= 404 then
        print("Unexpected status: " .. status)
    end
end

-- Print summary
function done(summary, latency, requests)
    io.write("\n")
    io.write("------------------------------\n")
    io.write("Load Test Summary\n")
    io.write("------------------------------\n")
    io.write(string.format("Total Requests:   %d\n", summary.requests))
    io.write(string.format("Total Duration:   %.2fs\n", summary.duration / 1000000))
    io.write(string.format("Requests/sec:     %.2f\n", summary.requests / (summary.duration / 1000000)))
    io.write(string.format("Total Errors:     %d\n", summary.errors.connect + summary.errors.read + summary.errors.write + summary.errors.timeout))
    io.write(string.format("Avg Latency:      %.2fms\n", latency.mean / 1000))
    io.write(string.format("Max Latency:      %.2fms\n", latency.max / 1000))
    io.write(string.format("Stdev Latency:    %.2fms\n", latency.stdev / 1000))
    io.write("\nLatency Distribution:\n")
    io.write(string.format("  50%%:  %.2fms\n", latency:percentile(50) / 1000))
    io.write(string.format("  75%%:  %.2fms\n", latency:percentile(75) / 1000))
    io.write(string.format("  90%%:  %.2fms\n", latency:percentile(90) / 1000))
    io.write(string.format("  99%%:  %.2fms\n", latency:percentile(99) / 1000))
    io.write("------------------------------\n")
end
