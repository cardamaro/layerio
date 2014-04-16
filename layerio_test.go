package layerio

import (
  "bytes"
  "compress/flate"
  "io"
  "io/ioutil"
  "sync"
  "testing"
)

func TestLayeredReaderSimple(t *testing.T) {
  src := new(bytes.Buffer)
  src.Write([]byte("00000"))

  wr, _ := flate.NewWriter(src, flate.DefaultCompression)
  wr.Write([]byte("111112222233333444445555566666777778888899999"))
  wr.Flush()

  r := NewAsyncLayeredReader(ioutil.NopCloser(src))
  defer r.Close()

  data := make([]byte, 5)
  var out string

  toggle := true
  for {
    n, err := r.Read(data)
    t.Logf("Read: %d err %v: %q", n, err, data)
    if err != nil {
      t.Logf("Error: %v", err)
      break
    }
    out += string(data[:n])
    if toggle {
      toggle = false
      r.Lock()
      r.R = flate.NewReader(r.R)
      r.Unlock()
    }
  }

  if out != "00000111112222233333444445555566666777778888899999" {
    t.Errorf("Wrong output: [%s]", out)
  }
}

func TestLayeredReaderPiped(t *testing.T) {
  src, w := io.Pipe()

  r := NewAsyncLayeredReader(src)

  data := make([]byte, 256)
  var out string
  var expected string
  var wg sync.WaitGroup
  wg.Add(1)

  go func() {
    for {
      n, err := r.Read(data)
      t.Logf("Read: %d err %v: %q", n, err, data[:n])
      if err != nil {
        t.Logf("Error: %v", err)
        break
      }
      out += string(data[:n])
      if string(data[:n]) == "3 COMPRESS DEFLATE\r\n" {
        r.Lock()
        r.R = flate.NewReader(r.R)
        r.Unlock()
      }
    }
    wg.Done()
  }()

  w.Write([]byte("1 LOGIN test test\r\n"))
  expected += "1 LOGIN test test\r\n"
  w.Write([]byte("2 CAPABILITY\r\n"))
  expected += "2 CAPABILITY\r\n"
  w.Write([]byte("3 COMPRESS DEFLATE\r\n"))
  expected += "3 COMPRESS DEFLATE\r\n"

  cw, _ := flate.NewWriter(w, flate.DefaultCompression)
  cw.Write([]byte("4 SELECT INBOX\r\n"))
  expected += "4 SELECT INBOX\r\n"
  cw.Flush()

  w.Close()
  wg.Wait()

  if out != expected {
    t.Fatalf("Wrong output: %s", out)
  }
}
