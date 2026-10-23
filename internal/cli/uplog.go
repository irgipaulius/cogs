package cli

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

const upLogMaxBytes = 2 << 20

var upLogWriter *os.File

func attachUpLog(root string) {
	path := upLogPath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	upLogWriter = f
	log.SetOutput(upLogWriterFor(f))
	writeUpLogStartMarker(f)
}

func writeUpLogStartMarker(f *os.File) {
	if f == nil {
		return
	}
	_, _ = fmt.Fprintf(f, "cogs up start pid=%d at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
}

func upLogBytesSinceCurrentRun(root string) []byte {
	path := upLogPath(root)
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return nil
	}
	marker := []byte("cogs up start pid=")
	idx := bytesLastIndex(b, marker)
	if idx < 0 {
		return b
	}
	return b[idx:]
}

func bytesLastIndex(b, sep []byte) int {
	for i := len(b) - len(sep); i >= 0; i-- {
		if string(b[i:i+len(sep)]) == string(sep) {
			return i
		}
	}
	return -1
}

func upLogWriterFor(f *os.File) io.Writer {
	if st, err := os.Stderr.Stat(); err == nil && (st.Mode()&os.ModeCharDevice) != 0 {
		return io.MultiWriter(os.Stderr, f)
	}
	return f
}

func rotateUpLogInProcess(root string) {
	if upLogWriter == nil {
		return
	}
	st, err := upLogWriter.Stat()
	if err != nil || st.Size() < upLogMaxBytes {
		return
	}
	path := upLogPath(root)
	_ = upLogWriter.Close()
	upLogWriter = nil
	for i := 2; i >= 0; i-- {
		src := path
		if i > 0 {
			src = path + "." + itoa(i)
		}
		dst := path + "." + itoa(i+1)
		if i == 2 {
			_ = os.Remove(dst)
		}
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	upLogWriter = f
	log.SetOutput(upLogWriterFor(f))
	writeUpLogStartMarker(f)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func closeUpLog() {
	if upLogWriter != nil {
		_ = upLogWriter.Close()
		upLogWriter = nil
	}
}
