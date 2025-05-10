package srpc_test

import (
	"context"
	"fmt"

	"github.com/changlongH/srpc/server"
	"github.com/cloudwego/hertz/pkg/app"
)

// @router backend/:cmd [POST]
func BackendProxy(ctx context.Context, c *app.RequestContext) {
	cmd := c.Param("cmd")
	if len(cmd) <= 0 {
		c.String(404, fmt.Sprintf("not found %s", cmd))
		return
	}

	// 消息分发
	var data = c.GetRawData()
	ret, err := server.GetDispatcher().DispatchReq("backend", cmd, data, false)
	if err != nil {
		c.String(500, fmt.Sprintf("InternalServerError: %s", err.Error()))
		return
	}

	if len(ret) > 0 {
		c.String(200, string(ret))
	} else {
		c.String(200, "Successful")
	}
}
