package main

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

var (
	fType        = flag.String("t", "direct", "proxy type (direct / socks5) [default: direct]")
	fPort        = flag.Int("r", 44444, "where to listen for receiver [default: 44444]")
	fGB          = flag.Int("n", 10, "number of gigabytes to transmit [default: 10]")
	fConcurrency = flag.Int("c", 1, "number of concurrect connections [default: 1]")
	fSocks5Port  = flag.Int("s", 1080, "socks5 port [default: 1080]")
)

func makeConnection() (net.Conn, error) {
	switch *fType {
	case "direct":
		return net.DialTCP("tcp4", nil, &net.TCPAddr{
			IP:   net.IP([]byte{127, 0, 0, 1}),
			Port: *fPort,
		})
	case "socks5":
		dialer, err := proxy.SOCKS5("tcp4", fmt.Sprintf("127.0.0.1:%d", *fPort), nil, proxy.Direct)
		if err != nil {
			return nil, err
		}
		return dialer.Dial("tcp4", fmt.Sprintf("127.0.0.1:%d", *fSocks5Port))
	default:
		return nil, errors.New("Unknown proxy type: " + *fType)
	}
}

func main() {
	flag.Parse()

	const BufSize = 128 * 1024
	var wg sync.WaitGroup

	startTime := time.Now().Unix()
	for i := 0; i < *fConcurrency; i++ {
		wg.Add(1)
		go func() {
			buf := make([]byte, BufSize)
			rand.Read(buf)
			conn, err := makeConnection()
			if err != nil {
				panic(err)
			}
			var connWg sync.WaitGroup
			connWg.Add(2)
			go func() {
				totalBytes := int64(*fGB) * 1024 * 1024 * 1024
				for totalBytes > 0 {
					_, err := conn.Write(buf)
					if err != nil {
						panic(err)
					}
					totalBytes -= BufSize
				}
				connWg.Done()
			}()
			go func() {
				totalBytes := int64(*fGB) * 1024 * 1024 * 1024
				for {
					var count uint64
					if err := binary.Read(conn, binary.BigEndian, &count); err != nil {
						panic(err)
					}
					if count >= uint64(totalBytes) {
						break
					}
				}
				connWg.Done()
			}()
			connWg.Wait()
			conn.Close()
			wg.Done()
		}()
	}
	wg.Wait()

	endTime := time.Now().Unix()
	elapsed := endTime - startTime
	if elapsed == 0 {
		fmt.Println("Finished in 0 second. Too fast.")
		return
	}
	dataAmount := uint64(*fConcurrency) * uint64(*fGB)

	speed := dataAmount * 1024 / uint64(elapsed)
	fmt.Println("Sender:", dataAmount, "GB of data sent through", *fConcurrency, "connections in", elapsed, "seconds, with speed", speed, "MB/s.")
}
