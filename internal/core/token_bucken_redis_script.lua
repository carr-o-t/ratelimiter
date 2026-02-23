local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local interval_ms = tonumber(ARGV[3])
local now_ms = tonumber(ARGV[4])
local ttl_ms = tonumber(ARGV[5])

local tokens = tonumber(redis.call("HGET", key, "tokens"))
local last_refill_ms = tonumber(redis.call("HGET", key, "last_refill_ms"))

if not tokens or not last_refill_ms then
  tokens = capacity
  last_refill_ms = now_ms
end

local elapsed = now_ms - last_refill_ms
if elapsed > 0 then
  local new_tokens = math.floor((elapsed * refill_rate) / interval_ms)
  if new_tokens > 0 then
    tokens = math.min(capacity, tokens + new_tokens)
    local consumed_ms = math.floor((new_tokens * interval_ms) / refill_rate)
    if consumed_ms < 0 then
      consumed_ms = 0
    end
    last_refill_ms = last_refill_ms + consumed_ms
  end
end

local allowed = 0
if tokens > 0 then
  tokens = tokens - 1
  allowed = 1
end

redis.call("HSET", key,
  "tokens", tokens,
  "last_refill_ms", last_refill_ms,
  "last_seen_ms", now_ms
)

if ttl_ms > 0 then
  redis.call("PEXPIRE", key, ttl_ms)
end

return {allowed, tokens}
