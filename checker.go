package main

import (
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/valyala/fasthttp"
	"strconv"
	"time"
)

type ReasonCode int

const (
	reasonInternalError    ReasonCode = -1
	reasonOk               ReasonCode = 0
	reasonForceEnabled     ReasonCode = 1
	reasonNodeNotAvailable ReasonCode = 2
	reasonWSRepFailed      ReasonCode = 3
	reasonCheckTimeout     ReasonCode = 4
	reasonRWDisabled       ReasonCode = 5
)

type Response struct {
	*NodeStatus
	ReasonText string
	ReasonCode ReasonCode
}

func checkerHandler(ctx *fasthttp.RequestCtx) {

	response := Response{NodeStatus: status}
	ctx.SetContentType("application/json")

	if config.CheckForceEnable {
		ctx.SetStatusCode(fasthttp.StatusOK)
		response.ReasonText = "Force enabled"
		response.ReasonCode = reasonForceEnabled
	} else if !status.NodeAvailable {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		response.ReasonText = "Node is not available"
		response.ReasonCode = reasonNodeNotAvailable
	} else if (status.WSRepStatus != 4) && (status.WSRepStatus != 2) {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		response.ReasonText = "WSRep failed"
		response.ReasonCode = reasonWSRepFailed
	} else if float64(status.Timestamp)+float64(config.CheckFailTimeout)/1000 < float64(time.Now().Unix()) {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		response.ReasonText = "Check timeout"
		response.ReasonCode = reasonCheckTimeout
	} else if !config.CheckROEnabled && !status.RWEnabled {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		response.ReasonText = "Node is read only"
		response.ReasonCode = reasonRWDisabled
	} else {
		ctx.SetStatusCode(fasthttp.StatusOK)
		response.ReasonText = "OK"
		response.ReasonCode = reasonOk
	}

	if respJson, err := json.Marshal(response); err != nil {
		errStr := fmt.Sprintf(`{"ReasonText":"Internal checker error","ReasonCode":%d,"err":"%s"}`, reasonInternalError, err)
		ctx.SetBody([]byte(errStr))
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	} else {
		ctx.SetBody(respJson)
	}

	return
}

func checker(status *NodeStatus) {
	for {
		time.Sleep(time.Duration(config.CheckInterval) * time.Millisecond)
		curStatus := &NodeStatus{}
		curStatus.Timestamp = int(time.Now().Unix())

		rows, err := dbConn.Query("SHOW GLOBAL VARIABLES;")
		if err != nil {
			*status = *curStatus
			continue
		}
		curStatus.NodeAvailable = true

		for rows.Next() {
			var key, value string
			err := rows.Scan(&key, &value)
			if err != nil {
				*status = *curStatus
				continue
			}
			switch key {
			case "read_only":
				if value == "OFF" {
					curStatus.RWEnabled = true
				}
			case "wsrep_local_state":
				curStatus.WSRepStatus, _ = strconv.Atoi(value)
			}
		}

		*status = *curStatus
	}
}
