package supervisor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client talks to the supervisor unix socket.
type Client struct {
	SocketPath string
}

// Call performs one RPC.
func (c *Client) Call(req Request) (Response, error) {
	conn, err := net.DialTimeout("unix", c.SocketPath, 3*time.Second)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	b, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	b = append(b, '\n')
	if _, err := conn.Write(b); err != nil {
		return Response{}, err
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	if !sc.Scan() {
		return Response{}, fmt.Errorf("supervisor: empty response")
	}
	var resp Response
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}
