-- constants
-- redis commands
local RCMD_EXPIRE = 'EXPIRE'
local RCMD_HGETALL = 'HGETALL'
local RCMD_HSET = 'HSET'
-- key's fields
local FIELD_START = 's'
local FIELD_TICK = 't'
local FIELD_INTERVAL = 'i'
local FIELD_CURRENT_TOKENS = 'k'
local FIELD_MAX_TOKENS = 'm'

-- script arguments
local key = KEYS[1]
-- now is current unix time in nanoseconds, its passed because time should be the time 
-- of calling not the time of invoking.
local now = tonumber(ARGV[1])
-- default tokens for interval. It should be used if there is no value exists for the key.
local defaultMaxTokens = tonumber(ARGV[2])
-- default interval in nanoseconds. Same as defaultmaxtokens.
local defaultInterval = tonumber(ARGV[3])

-- utility functions
local function hashGetAll(key)
    local data = redis.call(CMD_HGETALL, key)
    local result = {}

    for i = 1, #data, 2 do
        result[data[i]] = data[i + 1]
    end

    return result
end

local function availableTokens(lastTick, current, maxTokens, fillRate)
    local delta = current - lastTick
    local availableTkns = delta * fillRate

    if availableTkns > max then
        availableTkns = max
    end

    return availableTkns
end

local function isPresent(val)
    return val ~= nil and val ~= ''
end

local tick(start, current, interval)
    local count = math.floor((current - start) / interval)

    if count > 0 then
        return count
    end

    return 0
end

local timeToLive(interval)
    return 3 * math.floor(interval / 1000000000)
end

-- script begin
local data = hashGetAll(key)
local start = now
if isPresent(data[FIELD_START]) then
    start = tonumber(data[FIELD_START])
else
    redis.call(RCMD_HSET, key, FIELD_START, now)
    redis.call(RCMD_EXPIRE, key, 30)
end

local lastTick = 0
if isPresent(data[FIELD_TICK]) then
    lastTick = tonumber(data[FIELD_TICK])
else
    redis.call(RCMD_HSET, key, FIELD_TICK, 0)
    redis.call(RCMD_EXPIRE, key, 30)
end


local maxTokens = defaultMaxTokens
if isPresent(data[FIELD_MAX_TOKENS]) then
    maxTokens = tonumber(data[FIELD_MAX_TOKENS])
else
    redis.call(RCMD_HSET, key, FIELD_MAX_TOKENS, defaultMaxTokens)
    redis.call(RCMD_EXPIRE, key, 30)
end

local tokens = maxTokens
if isPresent(data[FIELD_CURRENT_TOKENS]) then
    tokens = tonumber(data[FIELD_CURRENT_TOKENS])
end

local interval = defaultInterval
if isPresent(data[FIELD_INTERVAL]) then
    interval = tonumber(data[FIELD_INTERVAL])
end

local currentTick = tick(start, now, interval)
local nextTime = start + ((currentTick + 1) * interval)

if lastTick > currentTick then
    local rate = interval / tokens
    tokens = availableTokens(lastTick, currentTick, maxTokens, rate)
    lastTick = currentTick
    redis.call(RCMD_HSET, key,
        FIELD_START, start,
        FIELD_TICK, lastTick,
        FIELD_INTERVAL, interval,
        FIELD_CURRENT_TOKENS, tokens
    )
    redis.call(RCMD_EXPIRE, key, timeToLive(interval))
end

if tokens > 0 then
    tokens = tokens - 1
    redis.call(RCMD_HSET, key, FIELD_CURRENT_TOKENS, tokens)
    redis.call(RCMD_EXPIRE, key, timeToLive(interval))

    return {maxTokens, tokens, nextTime, true}
end

return{maxTokens, tokens, nextTime, false}