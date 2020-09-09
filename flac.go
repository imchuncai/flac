package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	data, err := ioutil.ReadFile("a.flac")
	if err != nil {
		panic(err)
	}
	var steam = analyse(data)
	f, err := os.Create(`./out.txt`)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for _, metadata := range steam.Metadata {
		if metadata.BlockType == 4 {
			fmt.Println(ParseVorbis(metadata.Data))
		}
	}
}

func analyse(data []byte) (steam Steam) {
	copy(steam.Flac[:], data[:4])
	data = data[4:]
	var lastMetadataBlockFlag byte
	for lastMetadataBlockFlag != 1 {
		var metadata MetadataBlock
		var metadataBlockHeader = data[:4]
		data = data[4:]
		lastMetadataBlockFlag = metadataBlockHeader[0] >> 7
		metadata.LastMetadataBlock = lastMetadataBlockFlag == 1
		metadata.BlockType = int(metadataBlockHeader[0] & 0b0111_1111)
		var dataLength = int(metadataBlockHeader[1])<<16 + int(metadataBlockHeader[2])<<8 + int(metadataBlockHeader[3])
		metadata.Data = data[:dataLength]
		data = data[dataLength:]
		steam.Metadata = append(steam.Metadata, metadata)
	}
	steam.Frame = data
	return steam
}
