local skynet = require("skynet")
local cluster = require("skynet.cluster")
local srpc = require("srpc")

local db = srpc.new_dispatcher({ data = {} })
db.router("SETX", function(msg)
    local last = db.data[msg.key]
    db.data[msg.key] = msg.val
    return { succ = true, last = last }
end)

db.router("GETX", function(msg)
    if not msg.key then
        return nil
    end
    msg.val = db.data[msg.key]
    return msg
end)

db.router("SLEEP", function(ti)
    ti = (ti or 0) * 100
    skynet.sleep(ti)
    return ti
end)

skynet.start(function()
    skynet.dispatch("lua", function(session, addr, cmd, ...)
        cmd = cmd:upper()
        local f = db[cmd]
        if f then
            skynet.ret(skynet.pack(f(...)))
        else
            error(string.format("Unknown command %s", tostring(cmd)))
        end
    end)

    cluster.reload({
        db = "127.0.0.1:2528",
        db2 = "127.0.0.1:2529",
        golang = "127.0.0.1:2531",
    })

    cluster.register("sdb", skynet.self())
    cluster.open("db")
    cluster.open("db2")

    local node = "golang"
    local sname = "airth"
    srpc.send(node, sname, "Add", { a = 1, b = 2 })
    local ok, ret = srpc.call(node, sname, "Add", { a = 1, b = 2 })
    assert(ok and ret.c == 3)

    ok, ret = srpc.call(node, sname, "Div", { a = 4, b = 2 })
    assert(ok and ret.c == 2)

    ok, ret = srpc.call(node, sname, "Div", { a = 4, b = 0 })
    assert(not ok)

    ok, ret = srpc.call(node, sname, "Mul", { a = 2, b = 3 })
    assert(ok and ret.c == 6)

    ok, ret = srpc.call(node, sname, "Mul")
    assert(not ok, ret)

    ok, ret = srpc.call(node, sname, "Echo", "hello world")
    assert(ok and ret == "hello world", ret)

    ok, ret = srpc.call(node, sname, "Echo")
    assert(ok and ret == nil, ret)

    ok, ret = srpc.call(node, sname, "String", { a = 1, b = 2 })
    assert(ok and ret == "1+2=3", ret)

    ok, ret = srpc.call(node, sname, "Error")
    assert(not ok, ret)

    ok, ret = srpc.call(node, sname, "SleepMilli", 1000)
    assert(ok and ret == nil, ret)

    ok, ret = srpc.call(node, sname, "SleepMilli", 10.1)
    assert(not ok, ret)

    local values = { "", "1", "1.1", "{ a = 1 }" }
    ok, ret = srpc.call(node, sname, "EchoSlice", values)
    assert(ok, ret)
    for i, v in ipairs(values) do
        assert(v == ret[i], v)
    end

    local map = { a = 1, b = 2 }
    ok, ret = srpc.call(node, sname, "EchoMap", map)
    assert(ok, ret)
    for i, v in pairs(map) do
        assert(v == ret[i], v)
    end

    local msg = string.rep("a", 26)
    ok, ret = srpc.call(node, sname, "Echo", msg)
    assert(ok and ret == msg)

    msg = string.rep("a", 31)
    ok, ret = srpc.call(node, sname, "Echo", msg)
    assert(ok and ret == msg)

    msg = string.rep("a", 32)
    ok, ret = srpc.call(node, sname, "Echo", msg)
    assert(ok and ret == msg)

    msg = string.rep("a", 1000)
    ok, ret = srpc.call(node, sname, "Echo", msg)
    assert(ok and ret == msg)

    msg = string.rep("a", 0x10000)
    ok, ret = srpc.call(node, sname, "Echo", msg)
    assert(ok and ret == msg)

    msg = string.rep("a", 0x20000)
    ok, ret = srpc.call(node, sname, "Echo", msg)
    assert(ok and ret == msg)

    skynet.error("srpc call golang  successful")
end)