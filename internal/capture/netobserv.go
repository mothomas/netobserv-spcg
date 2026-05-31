package capture

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"time"
)

// NetObservRunner pipes oc-netobserv packets stdout into a memory-only reader.
type NetObservRunner struct {
	Bin string
}

func (n *NetObservRunner) BinPath() string {
	if n.Bin != "" {
		return n.Bin
	}
	if v := os.Getenv("NETOBSERV_BIN"); v != "" {
		return v
	}
	return "oc-netobserv"
}

// Start begins capture for a pod; returns stdout pipe reader and cancel func.
func (n *NetObservRunner) Start(ctx context.Context, namespace, podName string, port int32) (io.Reader, context.CancelFunc, error) {
	if namespace == "" || podName == "" {
		return nil, nil, fmt.Errorf("namespace and pod name are required for netobserv capture")
	}

	args := []string{
		"packets",
		"--namespace", namespace,
		"--pod", podName,
		"--output", "-",
	}
	if port > 0 {
		args = append(args, "--port", fmt.Sprintf("%d", port))
	}

	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, n.BinPath(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("failed creating netobserv stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("failed starting netobserv capture for pod %s/%s: %w", namespace, podName, err)
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		_, _ = io.Copy(pw, stdout)
		_ = cmd.Wait()
	}()

	go func() {
		<-cmdCtx.Done()
		_ = cmd.Process.Kill()
	}()

	return pr, cancel, nil
}

// ChunkReader reads fixed-size chunks for gRPC streaming with metrics.
type ChunkReader struct {
	r               io.Reader
	chunkSize       int
	sequence        uint64
	cumulativeBytes uint64
	lastWindow      time.Time
	windowBytes     uint64
	packetsPerSec   uint64
}

func NewChunkReader(r io.Reader, chunkSize int) *ChunkReader {
	if chunkSize <= 0 {
		chunkSize = 32 * 1024
	}
	return &ChunkReader{r: r, chunkSize: chunkSize, lastWindow: time.Now()}
}

func (c *ChunkReader) Next() ([]byte, uint64, uint64, uint64, error) {
	buf := make([]byte, c.chunkSize)
	n, err := c.r.Read(buf)
	if n > 0 {
		buf = buf[:n]
		atomic.AddUint64(&c.cumulativeBytes, uint64(n))
		c.windowBytes += uint64(n)
		now := time.Now()
		if now.Sub(c.lastWindow) >= time.Second {
			c.packetsPerSec = c.windowBytes
			c.windowBytes = 0
			c.lastWindow = now
		}
		c.sequence++
		return buf, c.sequence, c.packetsPerSec, c.cumulativeBytes, nil
	}
	if err != nil {
		return nil, 0, 0, c.cumulativeBytes, err
	}
	return nil, 0, c.packetsPerSec, c.cumulativeBytes, io.EOF
}

// DrainLines is used for subprocess stderr logging without persistence.
func DrainLines(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		// intentionally discarded — no disk writes
	}
}
