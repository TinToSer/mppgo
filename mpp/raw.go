package mpp

import (
	"io"
	"strings"

	"github.com/tintoser/mppgo/cfb"
)

func ReadRawStream(reader io.ReaderAt, path string, decode bool) ([]byte, error) {
	container, err := cfb.Open(reader)
	if err != nil {
		return nil, err
	}
	name := path
	if position := strings.LastIndex(path, "/"); position >= 0 {
		name = path[position+1:]
	}
	if !decode || (name != "Props" && name != "FixedData" && name != "Fixed2Data") {
		return container.OpenStream(path)
	}
	docRaw, err := container.OpenStream("Props14")
	if err != nil {
		return nil, err
	}
	props := ParseProps14(docRaw)
	if props.Byte(propsPasswordFlag)&1 != 0 && props.ByteArray(propsProtectionPasswordHash) != nil {
		return nil, ErrPasswordProtected
	}
	return newStreamSource(container, props).decoded(path)
}
