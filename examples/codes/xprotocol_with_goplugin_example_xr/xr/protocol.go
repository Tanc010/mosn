/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mosn.io/mosn/examples/codes/xprotocol_with_goplugin_example_xr/common"
	"mosn.io/mosn/examples/codes/xprotocol_with_goplugin_example_xr/common/safe"
	"mosn.io/api"
	"mosn.io/pkg/buffer"
	"mosn.io/pkg/log"
	"strconv"
	"strings"
	"sync/atomic"
)

// Proto protocol format: 8 byte length + string body
// <Service>
//    <Header>
//		 <key> ... </key>
//		 <...> ... </...>
//    </Header>
//    <Body>
//		 <key> ... </key>
//    </Body>
//  </Service>
//
// ------------------ request example ---------------------------
// EXT_REF: Business requests are replaced automatically
// RequestType, 0 request, 1 response
//
// <Service>
//    <Header>
//        <ServiceCode>CIMT000070</ServiceCode>
//        <ChannelId>C48</ChannelId>
//        <ExternalReference>'$EXT_REF'</ExternalReference>
//        <OriginalChannelId>C49</OriginalChannelId>
//        <OriginalReference>06221113270051159201000092010000</OriginalReference>
//        <RequestTime>20210622111327543</RequestTime>
//        <Version>1.0</Version>
//        <RequestType>0</RequestType>
//        <Encrypt>0</Encrypt>
//        <TradeDate>20210617</TradeDate>
//        <RequestBranchCode>CN0010001</RequestBranchCode>
//        <RequestOperatorId>FB.ICP.X01</RequestOperatorId>
//        <RequestOperatorType>1</RequestOperatorType>
//        <TermType>00000</TermType>
//        <TermNo>0000000000</TermNo>
//    </Header>
//    <Body>
//        <Request>
//            <CustNo>3001504094</CustNo>
//        </Request>
//    </Body>
//  </Service>

type Proto struct {
	streams safe.IntMap
}

func (proto *Proto) Name() api.ProtocolName {
	return Xr
}

func (proto *Proto) Encode(ctx context.Context, model interface{}) (api.IoBuffer, error) {
	switch frame := model.(type) {
	case *Request:
		return proto.encodeRequest(ctx, frame)
	case *Response:
		return proto.encodeResponse(ctx, frame)
	default:
		log.DefaultLogger.Errorf("[protocol][xr] encode with unknown command : %+v", model)
		return nil, errors.New("unknown command type")
	}
}

func (proto *Proto) Decode(ctx context.Context, buf api.IoBuffer) (interface{}, error) {

	bLen := buf.Len()
	data := buf.Bytes()

	if bLen < 8 /** sk header length*/ {
		return nil, nil
	}

	var packetLen = 0
	var err error

	rawLen := strings.TrimLeft(string(data[0:8]), "0")
	if rawLen != "" {
		packetLen, err = strconv.Atoi(rawLen)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to decode package len %d, err: %v", packetLen, err))
		}
	}

	// expected full message length
	if bLen < packetLen {
		return nil, nil
	}

	totalLen := 8 /** fixed 8 byte len */ + packetLen

	rpcHeader := common.Header{}
	injectHeaders(data[8:totalLen], &rpcHeader)

	frameType, _ := rpcHeader.Get(requestTypeKey)
	switch frameType {
	case requestFlag:
		return proto.decodeRequest(ctx, buf, &rpcHeader)
	case responseFlag:
		return proto.decodeResponse(ctx, buf, &rpcHeader)
	default:
		return nil, fmt.Errorf("decode xr rpc Error, unkownen request type = %s", frameType)
	}
}

// Trigger heartbeat detect.
func (proto *Proto) Trigger(context context.Context, requestId uint64) api.XFrame {
	return nil
}

func (proto *Proto) Reply(context context.Context, request api.XFrame) api.XRespFrame {
	return nil
}

// Hijack hijack request, maybe timeout
func (proto *Proto) Hijack(context context.Context, request api.XFrame, statusCode uint32) api.XRespFrame {
	resp := proto.hijackResponse(request, statusCode)

	return resp

}

func (proto *Proto) Mapping(httpStatusCode uint32) uint32 {
	return httpStatusCode
}

// PoolMode returns whether ping-pong or multiplex
func (proto *Proto) PoolMode() api.PoolMode {
	return api.Multiplex
}

func (proto *Proto) EnableWorkerPool() bool {
	return true
}

func (proto *Proto) GenerateRequestID(streamID *uint64) uint64 {
	return atomic.AddUint64(streamID, 1)
}

// hijackResponse build hijack response
func (proto *Proto) hijackResponse(request api.XFrame, statusCode uint32) *Response {
	req := request.(*Request)
	body := req.Payload.String()

	var bodyBuf bytes.Buffer
	headerIndex := strings.Index(body, startHeader)
	if headerIndex >= 0 {
		bodyBuf.WriteString(body[:headerIndex+len(startHeader)])
		bodyBuf.WriteString("<Response>")
		bodyBuf.WriteString("<ReturnCode>")
		bodyBuf.WriteString(strconv.Itoa(int(statusCode)))
		bodyBuf.WriteString("</ReturnCode>")
		bodyBuf.WriteString("<ReturnMessage>此请求被劫持</ReturnMessage>")
		bodyBuf.WriteString("</Response>")
		bodyBuf.WriteString(body[headerIndex+len(startHeader):])
	}
	body = bodyBuf.String()
	// replace request type -> response
	body = strings.ReplaceAll(body, "<RequestType>0</RequestType>", "<RequestType>1</RequestType>")

	// 8 byte length + string body
	buf := buffer.GetIoBuffer(8 + len(body))
	proto.prefixOfZero(buf, len(body))
	buf.WriteString(body)

	// response header
	rpcHeader := common.Header{}
	injectHeaders(buf.Bytes()[8:8+len(body)], &rpcHeader)

	resp := NewRpcResponse(&rpcHeader, buf)
	return resp
}
