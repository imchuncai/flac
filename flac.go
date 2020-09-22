package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var ERROR_NOT_FLAC = errors.New("Not a FLAC stream")

func main() {
	var steam, err = analyze("a.flac")
	if err != nil {
		panic(err)
	}
	fmt.Println(steam.VorbisComments)
	f, err := os.Create("temp.flac")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	steam.VorbisComments.UserCommentList["ARTIST"] = "imchucani"

	err = steam.Repack(f)
	if err != nil {
		panic(err)
	}
}

// func main() {
// 	var steam, err = analyze("a.flac")
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Println(steam.VorbisComments)
// }

func analyze(path string) (steam Steam, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() {
		err = f.Close()
	}()

	steam.Marker, err = readMarker(f)
	if err != nil || steam.Marker != [4]byte{'f', 'L', 'a', 'C'} {
		return steam, ERROR_NOT_FLAC
	}

	steam.MetadataBlock, steam.VorbisComments, err = readMetadataBlock(f)
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

func readMetadataBlock(f *os.File) (metadataBlock []Metadata, vorbisComments Vorbis, err error) {
	metadataBlock = make([]Metadata, 0, 6)
	for {
		metadata, last, err := readMetadata(f)
		if err != nil {
			return nil, Vorbis{}, ERROR_NOT_FLAC
		}
		if metadata.BlockType == 4 {
			vorbisComments, err = parseVorbis(metadata.Data)
			if err != nil {
				return nil, Vorbis{}, ERROR_NOT_FLAC
			}
		} else {
			metadataBlock = append(metadataBlock, metadata)
		}
		if last {
			return metadataBlock, vorbisComments, nil
		}
	}
}

func readMetadata(f *os.File) (metadata Metadata, last bool, err error) {
	var header = make([]byte, 4)
	_, err = f.Read(header)
	if err != nil {
		return
	}
	last = header[0]>>7 == 1
	metadata.BlockType = header[0] & 0b0111_1111
	var length = int(header[1])<<16 + int(header[2])<<8 + int(header[3])
	metadata.Data = make([]byte, length)
	_, err = f.Read(metadata.Data)
	return
}

func parseVorbis(data []byte) (vorbisComments Vorbis, err error) {
	var r = bytes.NewReader(data)
	vorbisComments.VendorString, err = readData(r)
	if err != nil {
		return
	}
	userCommentListLength, err := readLength(r)
	if err != nil {
		return
	}
	vorbisComments.UserCommentList = make(map[string]string, userCommentListLength)
	for i := 0; i < userCommentListLength; i++ {
		var userComment, err = readData(r)
		if err != nil {
			return Vorbis{}, err
		}
		k, v, err := analyzeUserComment(userComment)
		if err != nil {
			return Vorbis{}, err
		}
		vorbisComments.UserCommentList[k] = v
	}
	return
}

func readLength(r *bytes.Reader) (length int, err error) {
	var data = make([]byte, 4)
	_, err = r.Read(data)
	if err != nil {
		return
	}
	return int(binary.LittleEndian.Uint32(data)), nil
}

func readData(r *bytes.Reader) (value string, err error) {
	length, err := readLength(r)
	if err != nil {
		return
	}
	var data = make([]byte, length)
	_, err = r.Read(data)
	return string(data), nil
}

func analyzeUserComment(data string) (key, value string, err error) {
	var equalSignIndex = strings.Index(data, "=")
	if equalSignIndex == -1 {
		return "", "", errors.New("User comment bad format")
	}
	return data[:equalSignIndex], data[equalSignIndex+1:], nil
}
