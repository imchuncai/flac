package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

var ERROR_NOT_FLAC = errors.New("Not a FLAC stream")

func main() {
	steam, err := analyze("a.flac")
	if err != nil {
		panic(err)
	}
	for _, metadata := range steam.MetadataBlock {
		if metadata.BlockType == 4 {
			fmt.Println(ParseVorbis(metadata.Data))
		}
	}
}

func analyze(path string) (steam Steam, err error) {
	var f *os.File
	f, err = os.Open(path)
	if err != nil {
		return
	}

	steam.Marker, err = readMarker(f)
	if err != nil || steam.Marker != [4]byte{'f', 'L', 'a', 'C'} {
		return steam, ERROR_NOT_FLAC
	}

	steam.MetadataBlock, err = readMetadataBlock(f)
	if err != nil {
		return steam, ERROR_NOT_FLAC
	}

	steam.Frame, err = ioutil.ReadAll(f)
	if err != nil {
		return steam, ERROR_NOT_FLAC
	}

	return
}

func readMarker(f *os.File) (marker [4]byte, err error) {
	_, err = f.Read(marker[:])
	if err != nil {
		return marker, ERROR_NOT_FLAC
	}
	return marker, nil
}

func readMetadataBlock(f *os.File) ([]Metadata, error) {
	var metadataBlock = make([]Metadata, 0, 6)
	for {
		var metadata, err = readMetadata(f)
		if err != nil {
			return nil, ERROR_NOT_FLAC
		}
		metadataBlock = append(metadataBlock, metadata)
		if metadata.LastMetadataBlock {
			return metadataBlock, nil
		}
	}
}

func readMetadata(f *os.File) (metadata Metadata, err error) {
	var header = make([]byte, 4)
	_, err = f.Read(header)
	if err != nil {
		return
	}
	metadata.LastMetadataBlock = header[0]>>7 == 1
	metadata.BlockType = header[0] & 0b0111_1111
	var length = int(header[1])<<16 + int(header[2])<<8 + int(header[3])
	metadata.Data = make([]byte, length)
	_, err = f.Read(metadata.Data)
	return
}
