package main

import (
	"sync"
	"io"
	"fmt"
	"encoding/binary"
)

type ToyClient struct {
	mu sync.Mutex
	conn io.ReadWriteCloser
	xid int64
	pending map[int64] chan int32
}

func MakeToyClient(conn io.ReadWriteCloser) *ToyClient {
	tc := &ToyClient{}
	tc.conn = conn
	tc.xid = 1
	tc.pending = map[int64]chan int32{}
	go tc.Listener()
	return tc
}

func (tc *ToyClient) writeRequest(xid int64, procNum, arg int32 ) {
	binary.Write(tc.conn, binary.LittleEndian, xid)
	binary.Write(tc.conn, binary.LittleEndian, procNum)
	binary.Write(tc.conn, binary.LittleEndian, arg)
}

func (tc *ToyClient) Readreply() (int64, int32) {
	var xid int64
	var arg int32
	binary.Read(tc.conn, binary.LittleEndian, &xid)
	binary.Read(tc.conn, binary.LittleEndian, &arg)
	return xid, arg
}

func (tc *ToyClient) Call(procNum, arg int32) int32 {
	done := make(chan int32)
	tc.mu.Lock()
	xid := tc.xid
	tc.xid++
	tc.pending[xid] = done
	tc.writeRequest(xid, procNum, arg)
	tc.mu.Unlock()

	reply := <-done
	tc.mu.Lock()
	delete(tc.pending, xid)
	tc.mu.Unlock()

	return reply
}

func (tc *ToyClient) Listener() {
	for {
		xid, reply := tc.Readreply()
		fmt.Println("Xid is %v, arg is %v", xid, reply)
		tc.mu.Lock()
		ch, ok := tc.pending[xid]
		tc.mu.Unlock()
		if ok {
			ch <- reply
		}
	}
}


type ToyServer struct {
	mu sync.Mutex
	conn io.ReadWriteCloser
	handlers map[int32]func(int32)int32
}

func MakeToyServer(conn io.ReadWriteCloser) *ToyServer {
	ts := &ToyServer{}
	ts.conn = conn
	ts.handlers = map[int32](func(int32)int32) {}
	go ts.Dispatcher()
	return ts
}

func (ts *ToyServer) ReadRequest() (int64, int32, int32) {
	var xid int64
	var arg int32
	var procNum int32
	binary.Read(ts.conn, binary.LittleEndian, &xid)
	binary.Read(ts.conn, binary.LittleEndian, &procNum)
	binary.Read(ts.conn, binary.LittleEndian, &arg)
	return xid, procNum, arg
}

func (ts *ToyServer) WriteReply(xid int64, arg int32) {
	binary.Write(ts.conn, binary.LittleEndian, xid)
	binary.Write(ts.conn, binary.LittleEndian, arg)
}

func (ts *ToyServer) Dispatcher() {
	for {
		xid, procNum, arg := ts.ReadRequest()
		ts.mu.Lock()
		fn, ok := ts.handlers[procNum]
		ts.mu.Unlock()

		go func() {
			var reply int32
			if ok {
				reply = fn(arg)
			}
			ts.mu.Lock()
			ts.WriteReply(xid, reply)
			ts.mu.Unlock()
		}()
	}
}

type Pair struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (p Pair) Read(data []byte) (int, error) {
	return p.r.Read(data)
}
func (p Pair) Write(data []byte) (int, error) {
	return p.w.Write(data)
}
func (p Pair) Close() error {
	p.r.Close()
	return p.w.Close()
}

func main() {
	fmt.Println("Rpc")
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	cp := Pair{r: r1, w: w2}
	sp := Pair{r: r2, w: w1}

	tc := MakeToyClient(cp)
	ts := MakeToyServer(sp)
	ts.handlers[22] = func(a int32) int32 { return a + 1 }

	reply := tc.Call(22, 100)
	fmt.Println("Call return is %v", reply)
}

