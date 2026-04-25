package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestResolvePublicStaticPathAllowsOnlyPublicAssets(t *testing.T) {
	root := t.TempDir()

	allowed := []string{
		"/",
		"/privacy.html",
		"/styles.css",
		"/src/main.js",
	}
	for _, requestPath := range allowed {
		if _, ok := resolvePublicStaticPath(root, requestPath); !ok {
			t.Fatalf("expected %q to be publicly served", requestPath)
		}
	}

	blocked := []string{
		"/main.go",
		"/runtime-config.json",
		"/super-admin.local.json",
		"/.git/config",
		"/../super-admin.local.json",
		"/src/../main.go",
	}
	for _, requestPath := range blocked {
		if _, ok := resolvePublicStaticPath(root, requestPath); ok {
			t.Fatalf("expected %q to be blocked", requestPath)
		}
	}
}

func TestResolvePublicStaticPathKeepsResolvedPathInsideRoot(t *testing.T) {
	root := t.TempDir()

	fullPath, ok := resolvePublicStaticPath(root, "/src/main.js")
	if !ok {
		t.Fatalf("expected asset path to resolve")
	}
	expected := filepath.Join(root, "src", "main.js")
	if fullPath != expected {
		t.Fatalf("expected %q, got %q", expected, fullPath)
	}
}

func TestReadClientFrameRejectsOversizedPayloads(t *testing.T) {
	frame := makeMaskedClientTextFrame(bytes.Repeat([]byte("a"), maxClientFramePayload+1))
	conn := stubConn{Reader: bytes.NewReader(frame)}

	if _, _, err := readClientFrame(conn); err == nil {
		t.Fatalf("expected oversized client payload to be rejected")
	}
}

func TestReadClientFrameRejectsFragmentedFrames(t *testing.T) {
	frame := makeMaskedClientFrame(0x01, []byte(`{"type":"input"}`), false)
	conn := stubConn{Reader: bytes.NewReader(frame)}

	if _, _, err := readClientFrame(conn); err == nil {
		t.Fatalf("expected fragmented client frame to be rejected")
	}
}

func makeMaskedClientTextFrame(payload []byte) []byte {
	return makeMaskedClientFrame(0x01, payload, true)
}

func makeMaskedClientFrame(opcode byte, payload []byte, fin bool) []byte {
	mask := [4]byte{1, 2, 3, 4}
	header := []byte{opcode, 0x80}
	if fin {
		header[0] |= 0x80
	}

	switch {
	case len(payload) < 126:
		header[1] |= byte(len(payload))
	case len(payload) <= 0xffff:
		header[1] |= 126
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(len(payload)))
		header = append(header, ext[:]...)
	default:
		header[1] |= 127
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(len(payload)))
		header = append(header, ext[:]...)
	}

	maskedPayload := append([]byte(nil), payload...)
	for i := range maskedPayload {
		maskedPayload[i] ^= mask[i%len(mask)]
	}

	frame := append(header, mask[:]...)
	frame = append(frame, maskedPayload...)
	return frame
}

type stubConn struct {
	*bytes.Reader
}

func (c stubConn) Write(_ []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (c stubConn) Close() error                       { return nil }
func (c stubConn) LocalAddr() net.Addr                { return stubAddr("local") }
func (c stubConn) RemoteAddr() net.Addr               { return stubAddr("remote") }
func (c stubConn) SetDeadline(_ time.Time) error      { return nil }
func (c stubConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c stubConn) SetWriteDeadline(_ time.Time) error { return nil }

type stubAddr string

func (a stubAddr) Network() string { return "stub" }
func (a stubAddr) String() string  { return string(a) }
