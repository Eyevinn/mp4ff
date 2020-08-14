package bits

import (
	"io/ioutil"
	"testing"
)

func BenchmarkWrite(b *testing.B) {
	writer := NewWriter(ioutil.Discard)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writer.Write(0xff, 8); err != nil {
			b.Fatal(err)
		}
	}
}
