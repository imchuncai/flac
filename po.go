package main

import (
	"encoding/binary"
	"io"
)

type Steam struct {
	Marker         [4]byte
	MetadataBlock  []Metadata
	VorbisComments Vorbis
	StreamInfo     Metadata
	Frame          []byte
}

type Metadata struct {
	BlockType byte
	Data      []byte
}

type Vorbis struct {
	VendorString    string
	UserCommentList map[string]string
}

func (s *Steam) Repack(w io.Writer) error {
	var err = s.repackMarker(w)
	if err != nil {
		return err
	}

	err = s.repackStreamInfo(w)
	if err != nil {
		return err
	}

	err = s.repackVorbisComments(w)
	if err != nil {
		return err
	}

	err = s.repackMetadataBlock(w)
	if err != nil {
		return err
	}

	err = s.repackFrame(w)
	if err != nil {
		return err
	}

	return nil
}

func (s *Steam) repackStreamInfo(w io.Writer) error {
	var _, err = w.Write([]byte{s.StreamInfo.BlockType})
	if err != nil {
		return err
	}
	var length = make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(s.StreamInfo.Data)))
	_, err = w.Write(length[1:])
	if err != nil {
		return err
	}
	_, err = w.Write(s.StreamInfo.Data)
	if err != nil {
		return err
	}
	return nil
}

func (s *Steam) repackMarker(w io.Writer) error {
	var _, err = w.Write(s.Marker[:])
	if err != nil {
		return err
	}
	return nil
}

func (s *Steam) repackMetadataBlock(w io.Writer) (err error) {
	for i, metadata := range s.MetadataBlock {
		if i == len(s.MetadataBlock)-1 {
			_, err = w.Write([]byte{metadata.BlockType | 0b1000_0000})
		} else {
			_, err = w.Write([]byte{metadata.BlockType})
		}
		if err != nil {
			return err
		}
		var length = make([]byte, 4)
		binary.BigEndian.PutUint32(length, uint32(len(metadata.Data)))
		_, err = w.Write(length[1:])
		if err != nil {
			return err
		}
		_, err = w.Write(metadata.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Steam) repackVorbisComments(w io.Writer) error {
	var _, err = w.Write([]byte{0b00000100})
	if err != nil {
		return err
	}

	var length = 4 + len(s.VorbisComments.VendorString) + 4
	for k, v := range s.VorbisComments.UserCommentList {
		var userComment = k + "=" + v
		length += 4 + len(userComment)
	}

	var lengthData = make([]byte, 4)
	binary.BigEndian.PutUint32(lengthData, uint32(length))
	_, err = w.Write(lengthData[1:])
	if err != nil {
		return err
	}

	var venderLengthData = make([]byte, 4)
	binary.LittleEndian.PutUint32(venderLengthData, uint32(len(s.VorbisComments.VendorString)))
	_, err = w.Write(venderLengthData)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(s.VorbisComments.VendorString))
	if err != nil {
		return err
	}
	var userCommentListLengthData = make([]byte, 4)
	binary.LittleEndian.PutUint32(userCommentListLengthData, uint32(len(s.VorbisComments.UserCommentList)))
	_, err = w.Write(userCommentListLengthData)
	if err != nil {
		return err
	}
	for k, v := range s.VorbisComments.UserCommentList {
		var userComment = k + "=" + v
		var lengthData = make([]byte, 4)
		binary.LittleEndian.PutUint32(lengthData, uint32(len(userComment)))
		_, err = w.Write(lengthData)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(userComment))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Steam) repackFrame(w io.Writer) error {
	var _, err = w.Write(s.Frame)
	return err
}
