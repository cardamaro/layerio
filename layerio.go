package layerio

import (
  "compress/flate"
  "io"
  "sync"
)

const bufferSize = 4096

type AsyncLayeredReader struct {
  sync.Mutex
  R   io.ReadCloser
  W   *io.PipeWriter
  src io.ReadCloser
  buf []byte
  err error
}

func NewAsyncLayeredReader(src io.ReadCloser) *AsyncLayeredReader {
  pr, pw := io.Pipe()
  r := &AsyncLayeredReader{
    src: src,
    R:   pr,
    W:   pw,
    buf: make([]byte, bufferSize),
  }
  go r.pump()
  return r
}

func (r *AsyncLayeredReader) pump() {
  defer r.W.Close()
  for {
    n, err := r.src.Read(r.buf)
    if err != nil {
      r.err = err
      break
    }
    r.W.Write(r.buf[:n])
  }
}

func (r *AsyncLayeredReader) Read(data []byte) (n int, err error) {
  r.Lock()
  reader := r.R
  r.Unlock()
  return reader.Read(data)
}

func (r *AsyncLayeredReader) Close() error {
  if r.err != nil {
    return r.err
  }

  if err := r.src.Close(); err != nil {
    return err
  }

  if err := r.W.Close(); err != nil {
    return err
  }

  if err := r.R.Close(); err != nil {
    return err
  }

  return nil
}

type LayeredReadWriter struct {
  sync.Mutex
  r         *AsyncLayeredReader
  w         io.WriteCloser
  logPrefix string
  compress  *flate.Writer
}

func NewLayeredReadWriter(r io.ReadCloser, w io.WriteCloser) *LayeredReadWriter {
  return &LayeredReadWriter{
    r: NewAsyncLayeredReader(r),
    w: w,
  }
}

func (c *LayeredReadWriter) Read(p []byte) (n int, err error) {
  return c.r.Read(p)
}

func (c *LayeredReadWriter) Write(p []byte) (n int, err error) {
  var w io.Writer

  c.Lock()
  if c.compress != nil {
    defer c.compress.Flush()
    w = c.compress
  } else {
    w = c.w
  }
  c.Unlock()

  return w.Write(p)
}

func (c *LayeredReadWriter) EnableCompression() error {
  c.r.Lock()
  c.r.R = flate.NewReader(c.r.R)
  c.r.Unlock()

  c.Lock()
  defer c.Unlock()
  w, err := flate.NewWriter(c.w, flate.DefaultCompression)
  if err != nil {
    return err
  }
  c.compress = w

  return nil
}
