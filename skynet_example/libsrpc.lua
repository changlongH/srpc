local skynet = require("skynet")
local profile = require("skynet.profile")
local cluster = require("skynet.cluster")
local cjson = require("cjson")
local msgpack = require("msgpack")

local function new_json_codec()
    -- encode empty table as null is custom cjson api
    -- fork from cloudwu/cjson
    cjson.encode_empty_table_as_null(true)
    local encoder = {
        encode = cjson.encode,
        decode = cjson.decode,
    }
    return encoder
end

local function new_msgpack_codec()
    -- encode empty table as null is custom msgpack api
    -- see ./skynet_example/msgpack.lua
    msgpack.encode_empty_table_as_null(true)
    local encoder = {
        encode = msgpack.encode_one,
        decode = msgpack.decode_one,
    }
    return encoder
end

local srpc = {
    payload_codecs = {
        ["json"] = new_json_codec(),
        ["msgpack"] = new_msgpack_codec(),
    },
    codec = new_msgpack_codec(),
    profile = false,
}

function srpc.register_codec(name, codec)
    srpc.payload_codecs[name] = codec
end

function srpc.set_default_codec(name)
    local c = srpc.payload_codecs[name]
    if c then
        srpc.codec = c
        return true
    end
    return false
end

function srpc.send(node, sname, cmd, req)
    if req then
        req = srpc.codec.encode(req)
    end
    cluster.send(node, sname, cmd, req)
end

function srpc.call(node, sname, cmd, req)
    if req then
        req = srpc.codec.encode(req)
    end
    if srpc.profile then
        profile.start()
    end
    local ok, ret = pcall(cluster.call, node, sname, cmd, req)
    if not ok then
        return ok, ret
    end
    if srpc.profile then
        local cost = profile.stop()
        skynet.error(node, sname, cmd, cost)
    end
    if ret then
        ret = srpc.codec.decode(ret)
    end
    return ok, ret
end


function srpc.router(svc, cmd, callback)
    svc[cmd] = function(req)
        if req then
            req = srpc.codec.decode(req)
        end
        if srpc.profile then
            profile.start()
        end
        local ret = callback(req)
        if ret then
            ret = srpc.codec.encode(ret)
        end
        if srpc.profile then
            local cost = profile.stop()
            skynet.error(cmd, cost)
        end
        return ret
    end
end

return srpc