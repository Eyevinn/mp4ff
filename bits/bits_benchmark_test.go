package bits

import (
	"io/ioutil"
	"testing"
)

func BenchmarkWrite(b *testing.B) {
	writer := NewWriter(ioutil.Discard)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Write(0xff, 8)
	}
	err := writer.Error()
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkEbspWrite(b *testing.B) {
	writer := NewEBSPWriter(ioutil.Discard)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Write(0xff, 8)
	}
}
