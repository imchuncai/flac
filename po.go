package main

import (
	"encoding/binary"
	"strings"
)

type Steam struct {
	Flac     [4]byte
	Metadata []MetadataBlock
	Frame    []byte
}

type MetadataBlock struct {
	MetadataBlockHeader
	Data []byte
}

type MetadataBlockHeader struct {
	LastMetadataBlock bool
	BlockType         int
}

type Vorbis struct {
	VendorString    []byte
	UserCommentList map[string]string
}

func ParseVorbis(data []byte) Vorbis {
	var vorbis Vorbis
	vorbis.UserCommentList = make(map[string]string)
	var venderLength = int(binary.LittleEndian.Uint32(data[:4]))
	data = data[4:]
	vorbis.VendorString = data[:venderLength]
	data = data[venderLength:]
	var userCommentListLength = int(binary.LittleEndian.Uint32(data[:4]))
	data = data[4:]

	for i := 0; i < userCommentListLength; i++ {
		var length = int(binary.LittleEndian.Uint32(data[:4]))
		data = data[4:]
		var value = string(data[:length])
		data = data[length:]
		var k, v = AnalyzeKV(value)
		vorbis.UserCommentList[k] = v
	}
	return vorbis
}

func AnalyzeKV(data string) (key, value string) {
	var EqualSignIndex = strings.Index(data, "=")
	return data[:EqualSignIndex], data[EqualSignIndex+1:]
}

// func RepackStream(steam Steam, vorbis Vorbis) []byte {
// 	var data = make([]byte, 0, len(steam.Frame))
// 	data = append(data, steam.Flac[:]...)
// 	for _, metadata := range steam.Metadata {
// 		if metadata.BlockType != 4 {

// 			data = append(data)
// 		}
// 	}
// }
// func PackMetadata(metadata MetadataBlock) []byte {
// 	if metadata.BlockType == 4 {
// 		return PackVorbis(metadata.Data)
// 	}
// }

// func PackVorbis(vorbis Vorbis) []byte {
// 	var data []byte
// }
