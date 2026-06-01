package pcap

import (
	"bytes"
	"encoding/binary"
	"time"
)

const (
	blockTypeSHB uint32 = 0x0a0d0d0a
	blockTypeIDB uint32 = 1
	blockTypeEPB uint32 = 6

	linkTypeEthernet uint16 = 1
	snapLen          uint32 = 65535
)

// frameRecord is one netobserv PCA payload (typically a full Ethernet frame).
type frameRecord struct {
	Data []byte
	At   time.Time
}

// IsPCAPContainer reports whether b already begins with a pcap or pcapng container.
func IsPCAPContainer(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	magic := binary.LittleEndian.Uint32(b[:4])
	switch magic {
	case blockTypeSHB, 0xa1b2c3d4, 0xd4c3b2a1:
		return true
	default:
		return false
	}
}

// EncodePCAPng wraps raw link-layer frames in a minimal PCAP-NG file (SHB + IDB + EPBs).
func EncodePCAPng(frames []frameRecord) []byte {
	if len(frames) == 0 {
		return nil
	}
	var out bytes.Buffer
	writeSHB(&out)
	writeIDB(&out)
	base := time.Now()
	for i, fr := range frames {
		if len(fr.Data) == 0 {
			continue
		}
		ts := fr.At
		if ts.IsZero() {
			ts = base.Add(time.Duration(i) * time.Microsecond)
		}
		writeEPB(&out, fr.Data, ts)
	}
	return out.Bytes()
}

func writeSHB(buf *bytes.Buffer) {
	body := new(bytes.Buffer)
	_ = binary.Write(body, binary.LittleEndian, uint32(0x1a2b3c4d)) // byte-order magic
	_ = binary.Write(body, binary.LittleEndian, uint16(1))           // major
	_ = binary.Write(body, binary.LittleEndian, uint16(0))           // minor
	_ = binary.Write(body, binary.LittleEndian, int64(-1))            // section length
	_ = binary.Write(body, binary.LittleEndian, uint16(0))           // end of options
	_ = binary.Write(body, binary.LittleEndian, uint16(0))
	writeBlock(buf, blockTypeSHB, body.Bytes())
}

func writeIDB(buf *bytes.Buffer) {
	body := new(bytes.Buffer)
	_ = binary.Write(body, binary.LittleEndian, linkTypeEthernet)
	_ = binary.Write(body, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(body, binary.LittleEndian, snapLen)
	_ = binary.Write(body, binary.LittleEndian, uint16(0)) // end of options
	_ = binary.Write(body, binary.LittleEndian, uint16(0))
	writeBlock(buf, blockTypeIDB, body.Bytes())
}

func writeEPB(buf *bytes.Buffer, packet []byte, ts time.Time) {
	body := new(bytes.Buffer)
	_ = binary.Write(body, binary.LittleEndian, uint32(0)) // interface id
	us := ts.UnixMicro()
	_ = binary.Write(body, binary.LittleEndian, uint32(us>>32))
	_ = binary.Write(body, binary.LittleEndian, uint32(us&0xffffffff))
	captured := uint32(len(packet))
	_ = binary.Write(body, binary.LittleEndian, captured)
	_ = binary.Write(body, binary.LittleEndian, captured)
	_, _ = body.Write(packet)
	if pad := pad4(len(packet)); pad > 0 {
		_, _ = body.Write(make([]byte, pad))
	}
	_ = binary.Write(body, binary.LittleEndian, uint16(0)) // end of options
	_ = binary.Write(body, binary.LittleEndian, uint16(0))
	writeBlock(buf, blockTypeEPB, body.Bytes())
}

func writeBlock(buf *bytes.Buffer, blockType uint32, body []byte) {
	total := uint32(12 + len(body)) // type + len + body + trailing len
	_ = binary.Write(buf, binary.LittleEndian, blockType)
	_ = binary.Write(buf, binary.LittleEndian, total)
	_, _ = buf.Write(body)
	_ = binary.Write(buf, binary.LittleEndian, total)
}

func pad4(n int) int {
	return (4 - (n % 4)) % 4
}
