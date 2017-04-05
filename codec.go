package triplestore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

type Encoder interface {
	Encode(tris ...Triple) error
}

type Decoder interface {
	Decode() ([]Triple, error)
}

type datasetDecoder struct {
	newDecoderFunc func(io.Reader) Decoder
	rs             []io.Reader
}

func NewDatasetDecoder(fn func(io.Reader) Decoder, readers ...io.Reader) Decoder {
	return &datasetDecoder{newDecoderFunc: fn, rs: readers}
}

func (dec *datasetDecoder) Decode() ([]Triple, error) {
	type result struct {
		err  error
		tris []Triple
	}

	results := make(chan *result, len(dec.rs))

	var wg sync.WaitGroup
	for _, reader := range dec.rs {
		wg.Add(1)
		go func(r io.Reader) {
			defer wg.Done()
			tris, err := dec.newDecoderFunc(r).Decode()
			results <- &result{tris: tris, err: err}
		}(reader)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var all []Triple
	var errs []error
	for r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		}
		all = append(all, r.tris...)
	}

	if len(errs) > 0 {
		return all, errs[0]
	}

	return all, nil
}

type binaryEncoder struct {
	w io.Writer
}

type wordLength uint32

const (
	resourceTypeEncoding = uint8(0)
	literalTypeEncoding  = uint8(1)
)

func NewBinaryEncoder(w io.Writer) Encoder {
	return &binaryEncoder{w}
}

func (enc *binaryEncoder) Encode(tris ...Triple) error {
	for _, t := range tris {
		b, err := encodeTriple(t)
		if err != nil {
			return err
		}

		if _, err := enc.w.Write(b); err != nil {
			return err
		}
	}

	return nil
}

func encodeTriple(t Triple) ([]byte, error) {
	sub, pred := t.Subject(), t.Predicate()

	var buff bytes.Buffer

	binary.Write(&buff, binary.BigEndian, wordLength(len(sub)))
	buff.WriteString(sub)

	binary.Write(&buff, binary.BigEndian, wordLength(len(pred)))
	buff.WriteString(pred)

	obj := t.Object()
	if lit, isLit := obj.Literal(); isLit {
		binary.Write(&buff, binary.BigEndian, literalTypeEncoding)
		typ := lit.Type()
		binary.Write(&buff, binary.BigEndian, wordLength(len(typ)))
		buff.WriteString(string(typ))

		litVal := lit.Value()
		binary.Write(&buff, binary.BigEndian, wordLength(len(litVal)))
		buff.WriteString(litVal)
	} else {
		binary.Write(&buff, binary.BigEndian, resourceTypeEncoding)
		resID, _ := obj.ResourceID()
		binary.Write(&buff, binary.BigEndian, wordLength(len(resID)))
		buff.WriteString(resID)
	}

	return buff.Bytes(), nil
}

type binaryDecoder struct {
	r       io.Reader
	triples []Triple
}

func NewBinaryDecoder(r io.Reader) Decoder {
	return &binaryDecoder{r: r}
}

func (dec *binaryDecoder) Decode() ([]Triple, error) {
	for {
		done, err := dec.decodeTriple()
		if done {
			break
		} else if err != nil {
			return nil, err
		}
	}

	return dec.triples, nil
}

func (dec *binaryDecoder) decodeTriple() (bool, error) {
	sub, err := dec.readWord()
	if err == io.EOF {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("subject: %s", err)
	}

	pred, err := dec.readWord()
	if err != nil {
		return false, fmt.Errorf("predicate: %s", err)
	}

	var objType uint8
	if err := binary.Read(dec.r, binary.BigEndian, &objType); err != nil {
		return false, fmt.Errorf("object type: %s", err)
	}

	var decodedObj object
	if objType == resourceTypeEncoding {
		resource, err := dec.readWord()
		if err != nil {
			return false, fmt.Errorf("resource: %s", err)
		}
		decodedObj.resourceID = string(resource)

	} else {
		decodedObj.isLit = true
		var decodedLiteral literal

		litType, err := dec.readWord()
		if err != nil {
			return false, fmt.Errorf("literate type: %s", err)
		}
		decodedLiteral.typ = XsdType(litType)

		val, err := dec.readWord()
		if err != nil {
			return false, fmt.Errorf("literate: %s", err)
		}

		decodedLiteral.val = string(val)
		decodedObj.lit = decodedLiteral
	}

	dec.triples = append(dec.triples, &triple{
		sub:  subject(string(sub)),
		pred: predicate(string(pred)),
		obj:  decodedObj,
	})

	return false, nil
}

func (dec *binaryDecoder) readWord() ([]byte, error) {
	var len wordLength
	if err := binary.Read(dec.r, binary.BigEndian, &len); err != nil {
		return nil, err
	}

	word := make([]byte, len)
	if _, err := io.ReadFull(dec.r, word); err != nil {
		return nil, errors.New("triplestore: binary: cannot decode word")
	}

	return word, nil
}

type ntriplesEncoder struct {
	w io.Writer
}

func NewNTriplesEncoder(w io.Writer) Encoder {
	return &ntriplesEncoder{w}
}

func (enc *ntriplesEncoder) Encode(tris ...Triple) error {
	for _, t := range tris {
		var buff bytes.Buffer

		buff.WriteString(fmt.Sprintf("<%s> <%s> ", t.Subject(), t.Predicate()))
		if rid, ok := t.Object().ResourceID(); ok {
			buff.WriteString(fmt.Sprintf("<%s>", rid))
		}
		if lit, ok := t.Object().Literal(); ok {
			var litType string
			switch lit.Type() {
			case XsdBoolean:
				litType = "^^<http://www.w3.org/2001/XMLSchema#boolean>"
			case XsdDateTime:
				litType = "^^<http://www.w3.org/2001/XMLSchema#dateTime>"
			case XsdInteger:
				litType = "^^<http://www.w3.org/2001/XMLSchema#integer>"
			}
			buff.WriteString(fmt.Sprintf("\"%s\"%s", lit.Value(), litType))
		}
		buff.WriteString(" .\n")

		if _, err := enc.w.Write(buff.Bytes()); err != nil {
			return err
		}
	}

	return nil
}
