package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type DNSHeader struct {
	Id int64
	Flags int64
	NumOfQuestions int64
	NumOfAnswers int64
	NumOfAuthorities int64
	NumOfAdditional int64
}

func NewHeader(id, flags, questions, answers, authorities, additional int64) (dh *DNSHeader) {
	dh = &DNSHeader{
		Id: id,
		Flags: flags,
		NumOfQuestions: questions,
		NumOfAnswers: answers,
		NumOfAuthorities: authorities,
		NumOfAdditional: additional,
	}

	return dh
}


func (dh DNSHeader) ToBytes() (result []byte, err error) {
	var buf bytes.Buffer

	if err = binary.Write(&buf, binary.BigEndian, dh.Id); err != nil {
		err = fmt.Errorf("error writing Id to buffer: %v", err)
		return result, err
	}

	if err = binary.Write(&buf, binary.BigEndian, dh.Flags); err != nil {
		err = fmt.Errorf("error writing Flags to buffer: %v", err)
		return result, err
	}

	if err = binary.Write(&buf, binary.BigEndian, dh.NumOfQuestions); err != nil {
		err = fmt.Errorf("error writing 'Number of Questions' to buffer: %v", err)
		return result, err
	}

	if err = binary.Write(&buf, binary.BigEndian, dh.NumOfAnswers); err != nil {
		err = fmt.Errorf("error writing 'Number of Answers' to buffer: %v", err)
		return result, err
	}

	if err = binary.Write(&buf, binary.BigEndian, dh.NumOfAuthorities); err != nil {
		err = fmt.Errorf("error writing 'Number of Authorities' to buffer: %v", err)
		return result, err
	}

	if err = binary.Write(&buf, binary.BigEndian, dh.NumOfAdditional); err != nil {
		err = fmt.Errorf("error writing 'Number of Additional' to buffer: %v", err)
		return result, err
	}

	result = buf.Bytes()

	return result, err
}