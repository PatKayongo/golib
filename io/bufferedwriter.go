// Copyright (c) 2012 - Cloud Instruments Co., Ltd.
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
// ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package io

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// BufferedWriter stores data in memory and flushes it every flushPeriod or when buffer is full
type BufferedWriter struct {
	flushPeriod       time.Duration // data flushes interval (in microseconds)
	bufferMutex       *sync.Mutex   // mutex for buffer operations syncronization
	innerWriter       io.Writer     // inner writer
	buffer            *bufio.Writer // buffered wrapper for inner writer
	bufferSizeInBytes int           // max size of data chunk in bytes
}

// NewBufferedWriter creates a new buffered writer struct.
// bufferSizeInBytes -- size of memory buffer in bytes
// flushPeriod -- period in which data flushes from memory buffer in milliseconds. 0 - turn off this functionality
func NewBufferedWriter(innerWriter io.Writer, bufferSizeInBytes int, flushPeriod time.Duration) (*BufferedWriter, error) {

	if innerWriter == nil {
		return nil, errors.New("argument is nil: innerWriter")
	}
	if flushPeriod < 0 {
		return nil, fmt.Errorf("flushPeriod can not be less than 0. Got: %d", flushPeriod)
	}

	if bufferSizeInBytes <= 0 {
		return nil, fmt.Errorf("bufferSizeInBytes can not be less or equal to 0. Got: %d", bufferSizeInBytes)
	}

	buffer := bufio.NewWriterSize(innerWriter, bufferSizeInBytes)

	/*if err != nil {
		return nil, err
	}*/

	newWriter := new(BufferedWriter)

	newWriter.innerWriter = innerWriter
	newWriter.buffer = buffer
	newWriter.bufferSizeInBytes = bufferSizeInBytes
	newWriter.flushPeriod = flushPeriod * 1e6
	newWriter.bufferMutex = new(sync.Mutex)

	if flushPeriod != 0 {
		go newWriter.flushPeriodically()
	}

	return newWriter, nil
}

func (bufWriter *BufferedWriter) writeBigChunk(bytes []byte) (n int, err error) {
	bufferedLen := bufWriter.buffer.Buffered()

	n, err = bufWriter.flushInner()
	if err != nil {
		return
	}

	written, writeErr := bufWriter.innerWriter.Write(bytes)
	return bufferedLen + written, writeErr
}

// Sends data to buffer manager. Waits until all buffers are full.
func (bufWriter *BufferedWriter) Write(bytes []byte) (n int, err error) {

	bufWriter.bufferMutex.Lock()
	defer bufWriter.bufferMutex.Unlock()

	bytesLen := len(bytes)

	if bytesLen > bufWriter.bufferSizeInBytes {
		return bufWriter.writeBigChunk(bytes)
	}

	if bytesLen > bufWriter.buffer.Available() {
		n, err = bufWriter.flushInner()
		if err != nil {
			return
		}
	}

	bufWriter.buffer.Write(bytes)

	return len(bytes), nil
}

func (bufWriter *BufferedWriter) Close() error {
	closer, ok := bufWriter.innerWriter.(io.Closer)
	if ok {
		return closer.Close()
	}

	return nil
}

func (bufWriter *BufferedWriter) Flush() {

	bufWriter.bufferMutex.Lock()
	defer bufWriter.bufferMutex.Unlock()

	bufWriter.flushInner()
}

func (bufWriter *BufferedWriter) flushInner() (n int, err error) {
	bufferedLen := bufWriter.buffer.Buffered()
	flushErr := bufWriter.buffer.Flush()

	return bufWriter.buffer.Buffered() - bufferedLen, flushErr
}

func (bufWriter *BufferedWriter) flushBuffer() {
	bufWriter.bufferMutex.Lock()
	defer bufWriter.bufferMutex.Unlock()

	bufWriter.buffer.Flush()
}

func (bufWriter *BufferedWriter) flushPeriodically() {
	if bufWriter.flushPeriod > 0 {
		ticker := time.NewTicker(bufWriter.flushPeriod)
		for {
			<-ticker.C
			bufWriter.flushBuffer()
		}
	}
}

func (bufWriter *BufferedWriter) String() string {
	return fmt.Sprintf("BufferedWriter size: %d, flushPeriod: %d", bufWriter.bufferSizeInBytes, bufWriter.flushPeriod)
}
